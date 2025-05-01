package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

var (
	errOverCapacity = errors.New("cache: 超过容量限制")
)

// MaxCntCache 控制住緩存住的键值对数量
type MaxCntCache struct {
	*BuildInMapCache
	// 现在的键值对数量
	cnt int32
	// 最大的键值对数量
	maxCnt int32
}

/*
	只要key被删除就-1，set的时候需要考虑是否是更新值
*/

func NewMaxCntCache(c *BuildInMapCache, maxCnt int32) *MaxCntCache {
	res := &MaxCntCache{
		BuildInMapCache: c,
		maxCnt:          maxCnt,
	}
	origin := c.onEvicted

	res.onEvicted = func(k string, v any) {
		atomic.AddInt32(&res.cnt, -1)
		if origin != nil {
			origin(k, v)
		}
	}
	return res
}

func (c *MaxCntCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	_, ok := c.BuildInMapCache.data[key]
	if !ok {
		if c.cnt >= c.maxCnt {
			// 可以设计复杂的淘汰策略
			return errOverCapacity
		}
		// 在加了锁的情况下，不需要进行原子操作
		//atomic.AddInt32(&c.cnt, 1)
		c.cnt++
	}

	// 不能这样执行Set
	// 因为Set方法内部也需要拿锁，sync的锁是不可重入，会导致死锁
	//return c.BuildInMapCache.Set(ctx, key, val, expiration)

	// 所以我们需要抽取出业务逻辑，让里面不包含锁
	return c.set(key, val, expiration)
}
