package cache

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"time"
)

var (
	ErrFailedToPreemptLock = errors.New("redis-lock: 抢锁失败")
	ErrKeyNotHold          = errors.New("redis-lock: 你没有持有锁")

	//go:embed lua/unlock.lua
	luaUnlock string

	//go:embed lua/refresh_lock.lua
	luaRefresh string

	//go:embed lua/lock.lua
	luaLock string
)

type Client struct {
	client redis.Cmdable
	g      singleflight.Group
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{client: client}
}

// SingleflightLock singleflight优化
func (c *Client) SingleflightLock(ctx context.Context, key string, expiration time.Duration,
	timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	for {
		flag := false
		// DoChan返回的结果是你的方法返回的结果，异步执行并返回 channel
		resChan := c.g.DoChan(key, func() (interface{}, error) {
			flag = true
			return c.Lock(ctx, key, expiration, timeout, retry)
		})
		select {
		case res := <-resChan:
			if flag {
				// Forget 方法：移除 key 的调用记录
				c.g.Forget(key) // 下次调用 Do("key", fn) 将重新执行
				if res.Err != nil {
					return nil, res.Err
				}
				return res.Val.(*Lock), nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) TryLock(ctx context.Context, key string, expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	ok, err := c.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		// 代表别人抢到了锁，但不是该goroutine
		return nil, ErrFailedToPreemptLock
	}
	return &Lock{
		client: c.client,
		// 让外面传入key，方便解锁
		key:        key,
		val:        val,
		expiration: expiration,
	}, nil
}

func (c *Client) Lock(ctx context.Context, key string, expiration time.Duration,
	timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	var timer *time.Timer
	val := uuid.New().String()
	for {
		lctx, cancel := context.WithTimeout(ctx, timeout)
		// 在这里重试
		res, err := c.client.Eval(lctx, luaLock, []string{key}, val, expiration.Seconds()).Result()
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		// 加锁成功
		if res == "OK" {
			return &Lock{
				client: c.client,
				// 让外面传入key，方便解锁
				key:        key,
				val:        val,
				expiration: expiration,
			}, nil
		}

		interval, ok := retry.Next()
		if !ok {
			return nil, fmt.Errorf("redis-lock: 超出重试限制, %w", ErrFailedToPreemptLock)
		}
		if timer == nil {
			timer = time.NewTimer(interval)
		}

		select {
		case <-timer.C:
			// 什么都不用做，让他进入下一个循环
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

type Lock struct {
	client     redis.Cmdable
	key        string
	val        string
	expiration time.Duration
	unlockChan chan struct{}
}

func (l *Lock) UnLock(ctx context.Context) error {
	// 下面这一大堆，典型的check do something，
	// 所以 我们需要一个手段保证，在我们check和del的期间没有任何一个goroutine能进来
	// 做一个校验，防止解锁解到别人的了
	//val, err := l.client.Get(ctx, l.key).Result()
	//if err != nil {
	//	return err
	//}
	//if val != l.val {
	//	return errors.New("锁不是")
	//}
	//// 把键值对删除
	//cnt, err := l.client.Del(ctx, l.key).Result()
	//if err != nil {
	//	return err
	//}
	//if cnt != 1 {
	//	// 你的锁过期了或者出现其他问题
	//	// 哨兵错误，返回一个模糊的错误，用户愿意处理就处理
	//	return ErrKeyNotHold
	//}
	// 这就要使用到lua脚本，因为lua脚本是一个原子操作
	res, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.val).Int64()
	defer func() {
		select {
		case l.unlockChan <- struct{}{}:
		default:
		}
	}()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrKeyNotHold
	}
	return nil
}

func (l *Lock) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaRefresh, []string{l.key}, l.val, l.expiration).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrKeyNotHold
	}
	return nil
}

// AutoRefresh 自动续约
// interval 执行间隔
// timeout 超时时间
func (l *Lock) AutoRefresh(interval, timeout time.Duration) error {
	// 如果手动续约，那用户大概就是开个goroutine，然后使用下面代码
	// 自动续约的话，我们帮用户把这些部分提取出来，作为自动续约
	timeoutChan := make(chan struct{}, 1)
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutChan <- struct{}{}
				// continue 进入下次循环，让select进入timeout分支
				continue
			}
			if err != nil {
				return err
			}
		case <-timeoutChan:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			if errors.Is(err, context.DeadlineExceeded) {
				timeoutChan <- struct{}{}
				continue
			}
			if err != nil {
				return err
			}
		case <-l.unlockChan:
			// 收到停止信号，直接返回，结束循环
			return nil
		}
	}
}
