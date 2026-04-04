// Package nacos Nacos 配置中心接入
package nacos

import (
	"fmt"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"gopkg.in/yaml.v3"
)

// MasterConfig master 服务完整配置（从 Nacos 拉取）
type MasterConfig struct {
	MySQL struct {
		DSN string `yaml:"dsn"`
	} `yaml:"mysql"`

	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	JWT struct {
		Secret      string `yaml:"secret"`
		ExpireHours int    `yaml:"expire_hours"`
	} `yaml:"jwt"`

	COS struct {
		SecretID  string `yaml:"secret_id"`
		SecretKey string `yaml:"secret_key"`
		Bucket    string `yaml:"bucket"`
		Region    string `yaml:"region"`
	} `yaml:"cos"`

	SMS struct {
		SecretID   string `yaml:"secret_id"`
		SecretKey  string `yaml:"secret_key"`
		SDKAppID   string `yaml:"sdk_app_id"`
		SignName   string `yaml:"sign_name"`
		TemplateID string `yaml:"template_id"`
	} `yaml:"sms"`

	Nacos struct {
		Addr        string `yaml:"addr"`
		NamespaceID string `yaml:"namespace_id"`
		Group       string `yaml:"group"`
		ServiceName string `yaml:"service_name"`
	} `yaml:"nacos"`
}

// ConfigOptions Nacos 连接参数
type ConfigOptions struct {
	ServerAddr  string // e.g. "9.134.73.4"
	ServerPort  uint64 // e.g. 8848
	NamespaceID string // e.g. "dev"
	DataID      string // e.g. "master.yaml"
	Group       string // e.g. "DEFAULT_GROUP"
}

// LoadConfig 从 Nacos 拉取配置并解析为 MasterConfig
func LoadConfig(opts ConfigOptions) (*MasterConfig, error) {
	sc := []constant.ServerConfig{
		{
			IpAddr: opts.ServerAddr,
			Port:   opts.ServerPort,
		},
	}

	cc := constant.ClientConfig{
		NamespaceId:         opts.NamespaceID,
		TimeoutMs:           5000,
		NotLoadCacheAtStart: true,
		LogLevel:            "warn",
	}

	client, err := clients.NewConfigClient(vo.NacosClientParam{
		ClientConfig:  &cc,
		ServerConfigs: sc,
	})
	if err != nil {
		return nil, fmt.Errorf("create nacos config client: %w", err)
	}

	content, err := client.GetConfig(vo.ConfigParam{
		DataId: opts.DataID,
		Group:  opts.Group,
	})
	if err != nil {
		return nil, fmt.Errorf("get config from nacos (dataId=%s group=%s): %w", opts.DataID, opts.Group, err)
	}
	if content == "" {
		return nil, fmt.Errorf("nacos config is empty (dataId=%s group=%s)", opts.DataID, opts.Group)
	}

	var cfg MasterConfig
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("parse nacos config: %w", err)
	}
	return &cfg, nil
}

// WatchConfig 监听配置变更（热更新）
// onChange 在配置变更时被调用，参数为新的配置内容
func WatchConfig(client config_client.IConfigClient, opts ConfigOptions, onChange func(*MasterConfig)) error {
	return client.ListenConfig(vo.ConfigParam{
		DataId: opts.DataID,
		Group:  opts.Group,
		OnChange: func(namespace, group, dataId, data string) {
			var cfg MasterConfig
			if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
				return
			}
			onChange(&cfg)
		},
	})
}
