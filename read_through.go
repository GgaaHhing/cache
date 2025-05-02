package cache

import (
	"context"
	"errors"
	"time"
)

var (
	ErrFailedToRefreshCache = errors.New("刷新缓存失败")
)

// ReadThroughCache 读穿，由缓存层控制所有读取操作
// 你一定要赋值 LoadFunc 和 Expiration
type ReadThroughCache struct {
	Cache
	Expiration time.Duration
	LoadFunc   func(ctx context.Context, key string) (any, error)
}

func (r *ReadThroughCache) Get(ctx context.Context, key string) (any, error) {
	val, err := r.Cache.Get(ctx, key)
	if errors.Is(err, ErrKeyNotExists) {
		// 从数据库中取数据
		val, err = r.LoadFunc(ctx, key)
		if err == nil {
			// 取到了再放回去缓存中
			er := r.Cache.Set(ctx, key, val, r.Expiration)
			if er != nil {
				return nil, ErrFailedToRefreshCache
			}
		}
	}
	return val, err
}
