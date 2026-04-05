// Package nacos Worker 服务注册到 Nacos（供 Master 服务发现）
package nacos

import (
	"fmt"
	"os"
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

// Register 向 Nacos 注册 Worker 实例
// addr 格式: "ip:port"，e.g. "192.168.1.10:9090"
func Register(addr string) error {
	if namingClient == nil {
		return fmt.Errorf("nacos naming client not initialized")
	}
	ip, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.ParseUint(portStr, 10, 64)

	ok, err := namingClient.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          ip,
		Port:        port,
		ServiceName: serviceName,
		GroupName:   groupName,
		ClusterName: clusterName,
		Weight:      10,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true, // 心跳超时后自动摘除
		Metadata:    map[string]string{"version": "1.0"},
	})
	if err != nil {
		return fmt.Errorf("register worker instance: %w", err)
	}
	if !ok {
		return fmt.Errorf("register worker instance returned false")
	}
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
