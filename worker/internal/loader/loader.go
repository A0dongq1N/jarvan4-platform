// Package loader 从 COS 下载脚本 .so 并通过 Go plugin 加载
package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/Aodongq1n/jarvan4-platform/shared/cos"
	"github.com/Aodongq1n/jarvan4-platform/sdk/spec"
)

// ScriptLoader 脚本加载器（从 COS 下载 .so，plugin.Open 加载）
type ScriptLoader struct {
	cos      *cos.Client
	cacheDir string // 本地缓存目录
}

// New 创建脚本加载器
// cacheDir: 本地缓存根目录（默认 /tmp/worker-scripts）
func New(cosClient *cos.Client, cacheDir string) *ScriptLoader {
	if cacheDir == "" {
		cacheDir = "/tmp/worker-scripts"
	}
	return &ScriptLoader{cos: cosClient, cacheDir: cacheDir}
}

// Load 加载脚本：先尝试本地缓存，缓存未命中则从 COS 下载
// cosKey: COS 对象路径，e.g. "scripts/http_demo/a3f8c1d2.so"
// 返回的 ScriptEntry 可直接调用 Setup/Default/Teardown
func (l *ScriptLoader) Load(ctx context.Context, cosKey string) (spec.ScriptEntry, error) {
	localPath := filepath.Join(l.cacheDir, cosKey)

	// 检查本地缓存（按 cosKey 路径唯一，相同 commitHash 无需重复下载）
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		if err := l.download(ctx, cosKey, localPath); err != nil {
			return nil, err
		}
	}

	return l.openPlugin(localPath)
}

// download 从 COS 下载到本地
func (l *ScriptLoader) download(ctx context.Context, cosKey, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("loader: mkdir %s: %w", filepath.Dir(localPath), err)
	}
	if err := l.cos.DownloadFile(ctx, cosKey, localPath); err != nil {
		return fmt.Errorf("loader: download %s: %w", cosKey, err)
	}
	return nil
}

// openPlugin 用 Go plugin 加载 .so 并返回 ScriptEntry
func (l *ScriptLoader) openPlugin(localPath string) (spec.ScriptEntry, error) {
	p, err := plugin.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("loader: plugin.Open %s: %w", localPath, err)
	}

	sym, err := p.Lookup("Script")
	if err != nil {
		return nil, fmt.Errorf("loader: lookup 'Script' in %s: %w", localPath, err)
	}

	// 脚本导出格式：var Script spec.ScriptEntry = &XxxScript{}
	entry, ok := sym.(spec.ScriptEntry)
	if !ok {
		// 兼容导出为指针的情况：var Script = &XxxScript{}（未声明接口类型）
		if ptr, ok2 := sym.(*spec.ScriptEntry); ok2 {
			return *ptr, nil
		}
		return nil, fmt.Errorf("loader: Script symbol in %s does not implement spec.ScriptEntry", localPath)
	}
	return entry, nil
}
