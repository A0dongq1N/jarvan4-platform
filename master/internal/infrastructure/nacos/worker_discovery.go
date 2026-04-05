// Package nacos Worker 服务发现：订阅 Nacos stress-worker 服务实例变更
// 上线事件 → workerSvc.RegisterWorker
// 下线事件 → workerSvc.OfflineWorker
package nacos

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"

	appWorker "github.com/Aodongq1n/jarvan4-platform/master/internal/application/worker"
)

const (
	workerServiceName = "stress-worker"
	workerGroupName   = "DEFAULT_GROUP"
	workerClusterName = "default"
)

// WorkerDiscovery 订阅 Nacos stress-worker 服务，监听 Worker 上下线事件
type WorkerDiscovery struct {
	client    naming_client.INamingClient
	workerSvc *appWorker.Service
}

// NewWorkerDiscovery 创建 WorkerDiscovery
// addr: Nacos 地址，e.g. "9.134.73.4:8848"；namespace: Nacos 命名空间 ID
func NewWorkerDiscovery(addr, namespace string, workerSvc *appWorker.Service) (*WorkerDiscovery, error) {
	host, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.ParseUint(portStr, 10, 64)
	if port == 0 {
		port = 8848
	}

	sc := []constant.ServerConfig{{IpAddr: host, Port: port}}
	cc := constant.ClientConfig{
		NamespaceId:         namespace,
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogLevel:            "warn",
	}

	cli, err := clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: sc,
	})
	if err != nil {
		return nil, fmt.Errorf("create nacos naming client: %w", err)
	}

	return &WorkerDiscovery{client: cli, workerSvc: workerSvc}, nil
}

// Start 拉取当前全量实例 + 订阅后续变更事件
// 应在独立 goroutine 中调用（Subscribe 是异步回调，本函数返回后订阅持续生效）
func (d *WorkerDiscovery) Start(ctx context.Context) error {
	// 1. 拉取当前已注册的全量实例（Master 重启后感知存量 Worker）
	instances, err := d.client.SelectInstances(vo.SelectInstancesParam{
		ServiceName: workerServiceName,
		GroupName:   workerGroupName,
		Clusters:    []string{workerClusterName},
		HealthyOnly: true,
	})
	if err != nil {
		// 服务尚未有实例时返回错误，忽略，后续 Subscribe 会捕获上线事件
		fmt.Printf("[WorkerDiscovery] SelectInstances warning (may be empty): %v\n", err)
	}
	for _, inst := range instances {
		d.handleOnline(ctx, inst)
	}
	fmt.Printf("[WorkerDiscovery] loaded %d existing worker(s)\n", len(instances))

	// 2. 订阅后续变更（上线/下线/心跳超时自动摘除）
	// SubscribeCallback 收到的是变更后的完整实例列表
	if err := d.client.Subscribe(&vo.SubscribeParam{
		ServiceName: workerServiceName,
		GroupName:   workerGroupName,
		Clusters:    []string{workerClusterName},
		SubscribeCallback: func(services []model.Instance, err error) {
			if err != nil {
				fmt.Printf("[WorkerDiscovery] subscribe callback error: %v\n", err)
				return
			}
			d.reconcile(ctx, services)
		},
	}); err != nil {
		return fmt.Errorf("subscribe worker service: %w", err)
	}

	fmt.Println("[WorkerDiscovery] subscribed to stress-worker service")
	return nil
}

// reconcile 以 Nacos 当前实例列表为准，更新数据库
func (d *WorkerDiscovery) reconcile(ctx context.Context, instances []model.Instance) {
	fmt.Printf("[WorkerDiscovery] reconcile triggered: %d instance(s)\n", len(instances))
	for _, inst := range instances {
		if inst.Healthy && inst.Enable {
			d.handleOnline(ctx, inst)
		} else {
			d.handleOffline(ctx, inst.InstanceId)
		}
	}
}

func (d *WorkerDiscovery) handleOnline(ctx context.Context, inst model.Instance) {
	addr := fmt.Sprintf("%s:%d", inst.Ip, inst.Port)
	workerID := inst.InstanceId

	// 从 metadata 读取节点规格（Worker 注册时可写入，没有则用默认值）
	cpuCores, _ := strconv.Atoi(inst.Metadata["cpu_cores"])
	memGB, _ := strconv.ParseFloat(inst.Metadata["mem_gb"], 64)
	maxConcurrency, _ := strconv.Atoi(inst.Metadata["max_concurrency"])
	if cpuCores <= 0 {
		cpuCores = 4
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 500
	}

	if err := d.workerSvc.RegisterWorker(ctx, workerID, addr, cpuCores, memGB, maxConcurrency); err != nil {
		fmt.Printf("[WorkerDiscovery] RegisterWorker %s failed: %v\n", workerID, err)
	} else {
		fmt.Printf("[WorkerDiscovery] Worker online: id=%s addr=%s\n", workerID, addr)
	}
}

func (d *WorkerDiscovery) handleOffline(ctx context.Context, instanceID string) {
	if err := d.workerSvc.OfflineWorker(ctx, instanceID); err != nil {
		fmt.Printf("[WorkerDiscovery] OfflineWorker %s failed: %v\n", instanceID, err)
	} else {
		fmt.Printf("[WorkerDiscovery] Worker offline: id=%s\n", instanceID)
	}
}
