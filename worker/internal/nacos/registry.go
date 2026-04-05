// Package nacos Worker 服务注册到 Nacos（供 Master 服务发现）
package nacos

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

const serviceName = "stress-worker"
const groupName = "DEFAULT_GROUP"
const clusterName = "default"

var namingClient naming_client.INamingClient

// Init 初始化 Nacos 命名客户端（必须在 Register 前调用）
func Init() error {
	addr := getEnv("NACOS_ADDR", "9.134.73.4:8848")
	namespace := getEnv("NACOS_NAMESPACE", "7681a7b6-2c9a-4770-850f-b7c96bbdb7d1")

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

	var err error
	namingClient, err = clients.NewNamingClient(vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: sc,
	})
	if err != nil {
		return fmt.Errorf("create nacos naming client: %w", err)
	}
	return nil
}

// Register 向 Nacos 注册 Worker 实例，metadata 携带真实节点规格
// addr 格式: "ip:port"，e.g. "192.168.1.10:9090"
func Register(addr string) error {
	if namingClient == nil {
		return fmt.Errorf("nacos naming client not initialized")
	}
	ip, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.ParseUint(portStr, 10, 64)

	cpuCores := runtime.NumCPU()
	memGB := totalMemGB()
	maxConcurrency := cpuCores * 125 // 经验值：每核 125 并发

	ok, err := namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: serviceName,
		GroupName:   groupName,
		ClusterName: clusterName,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata: map[string]string{
			"version":         "1.0",
			"cpu_cores":       strconv.Itoa(cpuCores),
			"mem_gb":          strconv.FormatFloat(memGB, 'f', 1, 64),
			"max_concurrency": strconv.Itoa(maxConcurrency),
		},
	})
	if err != nil {
		return fmt.Errorf("register worker instance: %w", err)
	}
	if !ok {
		return fmt.Errorf("register worker instance returned false")
	}
	fmt.Printf("[Nacos] registered: addr=%s cpu=%d mem=%.1fGB maxConcurrency=%d\n",
		addr, cpuCores, memGB, maxConcurrency)
	return nil
}

// Deregister 从 Nacos 摘除 Worker 实例（进程退出时调用）
func Deregister(addr string) {
	if namingClient == nil {
		return
	}
	ip, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.ParseUint(portStr, 10, 64)

	_, _ = namingClient.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: serviceName,
		GroupName:   groupName,
		Ephemeral:   true,
	})
}

// totalMemGB 读取系统总内存（GB），Linux 读 /proc/meminfo，其他平台返回 0
func totalMemGB() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseFloat(fields[1], 64)
				return kb / 1024 / 1024 // kB → GB
			}
		}
	}
	return 0
}

// InstanceID 返回 Nacos 实例 ID（格式：{ip}#{port}#default#DEFAULT_GROUP@@stress-worker）
func InstanceID(addr string) string {
	ip, portStr, _ := strings.Cut(addr, ":")
	return fmt.Sprintf("%s#%s#%s#%s@@%s", ip, portStr, clusterName, groupName, serviceName)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
