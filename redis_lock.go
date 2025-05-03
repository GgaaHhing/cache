package cache

import (
	"context"
	_ "embed"
	"errors"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

var (
	ErrFailedToPreemptLock = errors.New("redis-lock: 抢锁失败")
	ErrKeyNotHold          = errors.New("redis-lock: 你没有持有锁")

	//go:embed lua/unlock.lua
	luaUnlock string
)

type Client struct {
	client redis.Cmdable
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
		key: key,
		val: val,
	}, nil
}

type Lock struct {
	client redis.Cmdable
	key    string
	val    string
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
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrKeyNotHold
	}
	return nil
}
