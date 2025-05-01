package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("cache: 没找到对应的key")
	// ErrKeyNotExists 无论是过期还是不存在，我们都默认当不存在返回，不需要解释
	ErrKeyNotExists = errors.New("cache: key不存在")
)

type BuildInMapCacheOption func(cache *BuildInMapCache)

func BuildInMapCacheWithEvictedCallBack(fn func(k string, v any)) BuildInMapCacheOption {
	return func(cache *BuildInMapCache) {
		cache.onEvicted = fn
	}
}

type BuildInMapCache struct {
	data map[string]*item
	// data sync.Map 无法做到精细的double check
	// 只要是读写问题，都要考虑并发和锁
	mutex sync.RWMutex
	close chan struct{}
	// 我们希望可以在进行删除操作的同时，打印出删除的全部信息
	onEvicted func(k string, v any)
}

func NewBuildInMapCache(size int, interval time.Duration, opts ...BuildInMapCacheOption) *BuildInMapCache {
	res := &BuildInMapCache{
		data:      make(map[string]*item, size),
		close:     make(chan struct{}),
		onEvicted: func(k string, v any) {},
	}

	// 放在goroutine之前
	for _, opt := range opts {
		opt(res)
	}

	// 策略2，缺点是如果key数量很多，一直没有遍历到一个在后面的已过期的key怎么办
	// 改进方案：结合策略3，在用户进行Get的时候再进行一次校验
	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			// 接收到.C的命令就是定时器触发，接收到的是ticker触发的时间，time.Time
			case t := <-ticker.C:
				res.mutex.Lock()
				i := 0
				for k, v := range res.data {
					// 用i来控制一次查询的数量
					if i > 1000 {
						break
					}
					// 我们假设如果Expiration=0，就是没有过期时间
					if v.deadlineBefore(t) {
						res.delete(k)
					}
					i++
				}
				res.mutex.Unlock()
			case <-res.close:
				ticker.Stop()
				return
			}
		}
	}()
	return res
}

func (b *BuildInMapCache) delete(k string) {
	itm, ok := b.data[k]
	if !ok {
		return
	}
	delete(b.data, k)
	b.onEvicted(k, itm.val)
}

// Set
/*
过期时间的几种策略：
	策路1：每个key 开一个goroutine 盯着，过期了就执行删除策路。
	策略2：用一个 goroutine 定时轮询，找到所有过期的 key，然后删除。
	策略3：什么都不干，用户访问key的时候，检查一下过期时间
*/
func (b *BuildInMapCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	err := b.set(key, val, expiration)
	if err != nil {
		return err
	}
	// 策略1，缺点是容易造成资源的浪费
	//time.AfterFunc(expiration, func() {
	//	b.mutex.Lock()
	//	defer b.mutex.Unlock()
	//	val, ok := b.data[key]
	//	if ok && val.deadline.Before(time.Now()) {
	//		delete(b.data, key)
	//	}
	//})

	return nil
}

// set 的提取是因为，我们希望可以将执行逻辑单独抽取出来，然后再进行加锁的操作
func (b *BuildInMapCache) set(key string, val any, expiration time.Duration) error {
	var dl time.Time
	// 如果expiration=0，则表示永不超时
	if expiration > 0 {
		dl = time.Now().Add(expiration)
	}
	b.data[key] = &item{
		val:      val,
		deadline: dl,
	}
	return nil
}

func (b *BuildInMapCache) Get(ctx context.Context, key string) (any, error) {
	b.mutex.RLock()
	// 后续校验有删除的部分，需要加写锁
	//defer b.mutex.RUnlock()
	res, ok := b.data[key]
	b.mutex.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	now := time.Now()
	if res.deadlineBefore(now) {
		b.mutex.Lock()
		defer b.mutex.Unlock()

		// double check 防止在我删除之前，又有goroutine重新设置了值
		res, ok = b.data[key]
		if !ok {
			return nil, ErrNotFound
		}
		if res.deadlineBefore(now) {
			b.delete(key)
		}

		return nil, ErrKeyNotExists
	}
	return res.val, nil
}

func (b *BuildInMapCache) Delete(ctx context.Context, key string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.delete(key)
	return nil
}

// Close 控制go routine的退出
func (b *BuildInMapCache) Close() error {
	//b.close <- struct{}{}
	// 做一个防止二次关闭的校验
	select {
	case b.close <- struct{}{}:
	default:
		return errors.New("cache：Close重复关闭")
	}
	return nil
}

// item item的作用是让对应的val有对应的过期时间
// 主要是为了应对，当用户在过期时间前修改了val，导致可能想要更新时间，却被旧的过期时间限制
type item struct {
	val      any
	deadline time.Time
}

func (i *item) deadlineBefore(t time.Time) bool {
	return !i.deadline.IsZero() && i.deadline.Before(t)
}
