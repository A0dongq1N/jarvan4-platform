// Package redis 实现执行实时状态缓存（使用 go-redis/v9 直连）
package redis

import (
	"context"
	"fmt"

	appExec "github.com/Aodongq1n/jarvan4-platform/master/internal/application/execution"
	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"
	goredisv9 "github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

// ExecutionCache 实现 app/execution.ExecutionCache
type ExecutionCache struct {
	client goredisv9.UniversalClient
}

// NewExecutionCacheFromConfig 直接使用 addr 和 password 创建缓存实例。
// 凭据从 Nacos 配置中心获取，不依赖 trpc_go.yaml。
func NewExecutionCacheFromConfig(addr, password string, db int) (*ExecutionCache, error) {
	cli := goredisv9.NewClient(&goredisv9.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	// 快速连通性验证
	if err := cli.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	return &ExecutionCache{client: cli}, nil
}

// 确保实现了接口
var _ appExec.ExecutionCache = (*ExecutionCache)(nil)

func (c *ExecutionCache) SetStatus(ctx context.Context, runID string, status int8) error {
	key := fmt.Sprintf("run:%s:status", runID)
	return c.client.Set(ctx, key, status, 0).Err()
}

func (c *ExecutionCache) SaveMetrics(ctx context.Context, runID string, payload *pbinternal.MetricsPayload) error {
	key := fmt.Sprintf("run:%s:metrics:latest", runID)
	data, err := proto.Marshal(payload)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, 0).Err()
}

func (c *ExecutionCache) GetLatestMetrics(ctx context.Context, runID string) (*pbinternal.MetricsPayload, error) {
	key := fmt.Sprintf("run:%s:metrics:latest", runID)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var payload pbinternal.MetricsPayload
	if err := proto.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
