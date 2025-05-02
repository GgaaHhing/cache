package cache

import (
	"context"
	"time"
)

// WriteThroughCache 在写入的时候，由Cache层来控制DB的刷入
type WriteThroughCache struct {
	Cache
	// 刷新DB
	StoreFunc func(ct context.Context, key string, val any) error
}

func (w *WriteThroughCache) Set(ctx context.Context, key string, val any, expiration time.Duration) error {
	err := w.StoreFunc(ctx, key, val)
	if err != nil {
		return err
	}
	return w.Cache.Set(ctx, key, val, expiration)
}
