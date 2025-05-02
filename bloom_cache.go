package cache

import "context"

type BloomFilterCache struct {
	ReadThroughCache
}

func NewBloomFilterCache(cache Cache, bf BloomFilter,
	loadfunc func(ctx context.Context, key string) (any, error)) *BloomFilterCache {
	return &BloomFilterCache{
		ReadThroughCache{
			Cache: cache,
			LoadFunc: func(ctx context.Context, key string) (any, error) {
				if !bf.HasKey(ctx, key) {
					return nil, ErrKeyNotExists
				}
				return loadfunc(ctx, key)
			},
		},
	}
}

type BloomFilter interface {
	HasKey(ctx context.Context, key string) bool
}
