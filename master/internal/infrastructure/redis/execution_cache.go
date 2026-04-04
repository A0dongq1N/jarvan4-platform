// Package redis 实现执行实时状态缓存（使用 trpc-database/goredis）
package redis

import (
	"context"
	"fmt"

	appExec "github.com/Aodongq1n/jarvan4-platform/master/internal/application/execution"
	pbinternal "github.com/Aodongq1n/jarvan4-platform/pb/masterinternal"
	goredisv9 "github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	goredis "trpc.group/trpc-go/trpc-database/goredis"
)

// ExecutionCache 实现 app/execution.ExecutionCache
type ExecutionCache struct {
	client goredisv9.UniversalClient
}

// NewExecutionCache 通过 trpc-database/goredis 创建缓存实例。
// 调用前须先通过 overrideRedisConfig 将 Nacos 注入的地址/密码覆盖到 trpc client 配置，
// 保证 goredis.New 读取的是真实凭据而非 trpc_go.yaml 占位值。
func NewExecutionCache(target string) (*ExecutionCache, error) {
	cli, err := goredis.New(target)
	if err != nil {
		return nil, err
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
