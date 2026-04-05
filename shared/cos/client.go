// Package cos 腾讯云 COS 对象存储客户端（shared，供 master 和 worker 共用）
package cos

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	cossdk "github.com/tencentyun/cos-go-sdk-v5"
)

// Client COS 客户端封装
type Client struct {
	cos    *cossdk.Client
	bucket string
	region string
}

// Config COS 初始化配置
type Config struct {
	SecretID  string
	SecretKey string
	Bucket    string // e.g. "jarvan4-scripts-1250000000"
	Region    string // e.g. "ap-guangzhou"
}

// NewClient 创建 COS 客户端
func NewClient(cfg Config) (*Client, error) {
	if cfg.SecretID == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("cos: SecretID and SecretKey are required")
	}
	if cfg.Bucket == "" || cfg.Region == "" {
		return nil, fmt.Errorf("cos: Bucket and Region are required")
	}

	bucketURL := fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region)
	u, err := url.Parse(bucketURL)
	if err != nil {
		return nil, fmt.Errorf("cos: parse bucket url: %w", err)
	}

	c := cossdk.NewClient(&cossdk.BaseURL{BucketURL: u}, &http.Client{
		Timeout: 60 * time.Second,
		Transport: &cossdk.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &Client{cos: c, bucket: cfg.Bucket, region: cfg.Region}, nil
}

// UploadFile 上传本地文件到 COS
// key: 对象存储路径，e.g. "scripts/http_login/a3f8c1d2.so"
// localPath: 本地文件路径
// 自动使用分块上传（>5MB），支持断点续传
func (c *Client) UploadFile(ctx context.Context, key, localPath string) error {
	_, _, err := c.cos.Object.Upload(ctx, key, localPath, &cossdk.MultiUploadOptions{
		OptIni: &cossdk.InitiateMultipartUploadOptions{
			ObjectPutHeaderOptions: &cossdk.ObjectPutHeaderOptions{
				ContentType: "application/octet-stream",
			},
		},
		PartSize:    10, // 每块 10MB
		ThreadPoolSize: 3,
	})
	if err != nil {
		return fmt.Errorf("cos upload %s → %s: %w", localPath, key, err)
	}
	return nil
}

// UploadBytes 将内存中的字节数据上传到 COS（小文件用，如元数据 JSON）
func (c *Client) UploadBytes(ctx context.Context, key string, data []byte) error {
	r, w := io.Pipe()
	go func() {
		_, _ = w.Write(data)
		w.Close()
	}()
	_, err := c.cos.Object.Put(ctx, key, r, &cossdk.ObjectPutOptions{
		ObjectPutHeaderOptions: &cossdk.ObjectPutHeaderOptions{
			ContentLength: int64(len(data)),
			ContentType:   "application/octet-stream",
		},
	})
	if err != nil {
		return fmt.Errorf("cos put %s: %w", key, err)
	}
	return nil
}

// DownloadFile 从 COS 下载文件到本地路径
// 自动创建目录，目标文件存在时覆盖
func (c *Client) DownloadFile(ctx context.Context, key, localPath string) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("cos: mkdir %s: %w", filepath.Dir(localPath), err)
	}

	_, err := c.cos.Object.Download(ctx, key, localPath, &cossdk.MultiDownloadOptions{
		ThreadPoolSize: 5,
		PartSize:       10, // 每块 10MB
	})
	if err != nil {
		return fmt.Errorf("cos download %s → %s: %w", key, localPath, err)
	}
	return nil
}

// GetPresignURL 生成预签名下载 URL（有效期内无需鉴权，供 Worker 拉取脚本使用）
// expiry: URL 有效期，建议 15 分钟（Worker 启动后立即拉取）
func (c *Client) GetPresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	presignURL, err := c.cos.Object.GetPresignedURL(ctx, http.MethodGet, key,
		c.cos.GetCredential().SecretID,
		c.cos.GetCredential().SecretKey,
		expiry, nil,
	)
	if err != nil {
		return "", fmt.Errorf("cos presign %s: %w", key, err)
	}
	return presignURL.String(), nil
}

// Exists 检查对象是否存在
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.cos.Object.Head(ctx, key, nil)
	if err != nil {
		// COS SDK 返回 404 时 err 包含状态码
		if cosErr, ok := err.(*cossdk.ErrorResponse); ok && cosErr.Response.StatusCode == 404 {
			return false, nil
		}
		return false, fmt.Errorf("cos head %s: %w", key, err)
	}
	return true, nil
}

// Delete 删除对象
func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.cos.Object.Delete(ctx, key, nil)
	if err != nil {
		return fmt.Errorf("cos delete %s: %w", key, err)
	}
	return nil
}

// ScriptKey 生成脚本产物在 COS 中的标准路径
// 格式：scripts/{scriptName}/{commitHash}.so
// e.g. scripts/http_login/a3f8c1d2e4b5.so
func ScriptKey(scriptName, commitHash string) string {
	return fmt.Sprintf("scripts/%s/%s.so", scriptName, commitHash)
}
