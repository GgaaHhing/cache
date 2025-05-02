package cache

import (
	"context"
	"golang.org/x/sync/singleflight"
	"time"
)

type SingleflightCache struct {
	ReadThroughCache
}

// NewSingleflightCache 无侵入式的Singlefligth
func NewSingleflightCache(cache Cache,
	loadFunc func(ctx context.Context, key string) (any, error), expiration time.Duration) *SingleflightCache {
	g := &singleflight.Group{}
	return &SingleflightCache{
		ReadThroughCache{
			Cache: cache,
			LoadFunc: func(ctx context.Context, key string) (any, error) {
				val, err, _ := g.Do(key, func() (any, error) {
					return loadFunc(ctx, key)
				})
				return val, err
			},
			Expiration: expiration,
		},
	}
}
