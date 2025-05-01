package cache

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"web/cache/mocks"
)

func TestRedis_Set(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testCases := []struct {
		name       string
		key        string
		val        any
		expiration time.Duration
		wantErr    error
		mock       func() redis.Cmdable
	}{
		{
			name: "Set success",
			mock: func() redis.Cmdable {
				// 用于模拟 Redis 命令返回状态的代码
				status := redis.NewStatusCmd(context.Background())
				status.SetVal("OK")
				cmd := mocks.NewMockCmdable(ctrl)
				cmd.EXPECT().Set(context.Background(), "key1", "value1", 10*time.Second).Return(status)
				return cmd
			},
			key:        "key1",
			val:        "value1",
			expiration: 10 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewRedisCache(tc.mock())
			err := c.Set(context.Background(), tc.key, tc.val, tc.expiration)
			assert.Equal(t, tc.wantErr, err)
		})
	}
}
