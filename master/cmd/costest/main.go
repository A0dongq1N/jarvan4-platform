// cos_test.go — COS 上传/下载集成测试（需真实 Nacos + COS 配置）
// 运行方式：
//   cd jarvan4-platform/master
//   GOWORK=off go run ./cmd/costest/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	infraCOS "github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/cos"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/infrastructure/nacos"
)

func main() {
	// 1. 从 Nacos 加载配置
	fmt.Println("=== 1. 从 Nacos 加载配置 ===")
	cfg, err := nacos.LoadConfig(nacos.ConfigOptions{
		ServerAddr:  "9.134.73.4",
		ServerPort:  8848,
		NamespaceID: "7681a7b6-2c9a-4770-850f-b7c96bbdb7d1",
		DataID:      "master.yaml",
		Group:       "DEFAULT_GROUP",
	})
	if err != nil {
		fmt.Printf("✗ Nacos 加载失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Nacos 配置加载成功\n")
	fmt.Printf("  COS Bucket: %s\n", cfg.COS.Bucket)
	fmt.Printf("  COS Region: %s\n", cfg.COS.Region)
	fmt.Printf("  COS SecretID: %s***\n", cfg.COS.SecretID[:6])

	// 2. 初始化 COS 客户端
	fmt.Println("\n=== 2. 初始化 COS 客户端 ===")
	cosClient, err := infraCOS.NewClient(infraCOS.Config{
		SecretID:  cfg.COS.SecretID,
		SecretKey: cfg.COS.SecretKey,
		Bucket:    cfg.COS.Bucket,
		Region:    cfg.COS.Region,
	})
	if err != nil {
		fmt.Printf("✗ COS 客户端初始化失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ COS 客户端初始化成功")

	ctx := context.Background()
	testKey := fmt.Sprintf("test/cos-test-%d.txt", time.Now().Unix())
	testContent := fmt.Sprintf("COS 上传/下载测试 - %s", time.Now().Format(time.RFC3339))

	// 3. 上传字节数据
	fmt.Printf("\n=== 3. 上传文本数据 (key: %s) ===\n", testKey)
	err = cosClient.UploadBytes(ctx, testKey, []byte(testContent))
	if err != nil {
		fmt.Printf("✗ 上传失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ 上传成功")

	// 4. 检查对象存在
	fmt.Println("\n=== 4. 检查对象是否存在 ===")
	exists, err := cosClient.Exists(ctx, testKey)
	if err != nil {
		fmt.Printf("✗ 检查失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 对象存在: %v\n", exists)

	// 5. 下载到本地文件
	localPath := "/tmp/cos-test-download.txt"
	fmt.Printf("\n=== 5. 下载到本地 (%s) ===\n", localPath)
	err = cosClient.DownloadFile(ctx, testKey, localPath)
	if err != nil {
		fmt.Printf("✗ 下载失败: %v\n", err)
		os.Exit(1)
	}
	downloaded, _ := os.ReadFile(localPath)
	if strings.TrimSpace(string(downloaded)) == strings.TrimSpace(testContent) {
		fmt.Printf("✓ 下载成功，内容一致: %s\n", string(downloaded))
	} else {
		fmt.Printf("✗ 内容不一致!\n  期望: %s\n  实际: %s\n", testContent, string(downloaded))
		os.Exit(1)
	}

	// 6. 生成预签名 URL
	fmt.Println("\n=== 6. 生成预签名 URL (15分钟有效) ===")
	presignURL, err := cosClient.GetPresignURL(ctx, testKey, 15*time.Minute)
	if err != nil {
		fmt.Printf("✗ 生成预签名 URL 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 预签名 URL 生成成功\n  %s\n", presignURL[:80]+"...")

	// 7. 上传本地文件（模拟 .so 脚本产物）
	soPath := "/tmp/test-script.so"
	os.WriteFile(soPath, []byte("fake .so binary content for testing"), 0644)
	soKey := infraCOS.ScriptKey("http_login", "a3f8c1d2e4b5")
	fmt.Printf("\n=== 7. 上传模拟脚本产物 (key: %s) ===\n", soKey)
	err = cosClient.UploadFile(ctx, soKey, soPath)
	if err != nil {
		fmt.Printf("✗ 脚本上传失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 脚本上传成功\n")

	// 8. 下载脚本产物（模拟 Worker 拉取）
	downloadedSo := "/tmp/plugins/http_login.so"
	fmt.Printf("\n=== 8. 下载脚本产物 (模拟 Worker 拉取) ===\n")
	err = cosClient.DownloadFile(ctx, soKey, downloadedSo)
	if err != nil {
		fmt.Printf("✗ 脚本下载失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 脚本下载成功: %s\n", downloadedSo)

	// 9. 清理测试文件
	fmt.Println("\n=== 9. 清理测试对象 ===")
	for _, key := range []string{testKey, soKey} {
		if err := cosClient.Delete(ctx, key); err != nil {
			fmt.Printf("⚠ 删除 %s 失败: %v\n", key, err)
		} else {
			fmt.Printf("✓ 已删除: %s\n", key)
		}
	}
	os.Remove(localPath)
	os.Remove(soPath)
	os.Remove(downloadedSo)

	fmt.Println("\n✅ 所有 COS 操作测试通过！")
}
