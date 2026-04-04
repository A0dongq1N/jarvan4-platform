//go:build ignore

// 示例压测脚本：HTTP 登录 + 查询商品接口
//
// 脚本规范：
//   - package main，必须导出 var Script spec.ScriptEntry
//   - 禁止独立 goroutine，禁止 os.Exit()
//   - 只允许 import 标准库 + github.com/Aodongq1n/jarvan4-platform/sdk/...
//
// 构建命令（与 Worker 使用相同镜像）：
//   go build -buildmode=plugin -o http_login.so .
//
// 环境变量（在任务绑定脚本时配置）：
//   BASE_URL   被压测服务地址，如 http://staging.example.com
//   USERNAME   登录账号
//   PASSWORD   登录密码
package main

import (
	"fmt"

	sdkhttp "github.com/Aodongq1n/jarvan4-platform/sdk/http"
	"github.com/Aodongq1n/jarvan4-platform/sdk/spec"
)

// Script 导出符号，Worker 通过 plugin.Lookup("Script") 获取。
var Script spec.ScriptEntry = &httpLoginScript{}

type httpLoginScript struct{}

// setupData Setup 阶段返回的共享数据，所有 VU 只读访问。
type setupData struct {
	BaseURL string
}

// Setup 在压测开始前执行一次。
// 验证环境变量，返回所有 VU 共享的配置数据。
func (s *httpLoginScript) Setup(ctx *spec.RunContext) (interface{}, error) {
	baseURL := ctx.Vars.Env("BASE_URL")
	if baseURL == "" {
		return nil, fmt.Errorf("BASE_URL 环境变量未配置")
	}

	// 验证服务可达性
	cli := sdkhttp.New(ctx, 0)
	resp, err := cli.Get(baseURL+"/health", sdkhttp.WithName("/health"))
	if err != nil {
		return nil, fmt.Errorf("服务不可达 %s: %w", baseURL, err)
	}
	ctx.Log.Info("Setup: 服务健康检查通过，状态码 %d", resp.StatusCode)

	return &setupData{BaseURL: baseURL}, nil
}

// Default 每次迭代调用一次，模拟用户登录 → 查询商品。
func (s *httpLoginScript) Default(ctx *spec.RunContext) error {
	sd := ctx.SetupData.(*setupData)
	baseURL := sd.BaseURL
	cli := sdkhttp.New(ctx, 0)

	// Step 1: 登录获取 token
	username := ctx.Vars.Env("USERNAME")
	password := ctx.Vars.Env("PASSWORD")
	if username == "" {
		username = fmt.Sprintf("user_%d", ctx.VUId)
	}
	if password == "" {
		password = "password123"
	}

	loginResp, err := cli.Post(baseURL+"/api/auth/login",
		map[string]string{"username": username, "password": password},
		sdkhttp.WithName("/api/auth/login"),
	)
	if err != nil {
		return err
	}

	// 断言登录成功
	ctx.Check.That(loginResp).Status(200).BodyJSON("code", "0")
	if ctx.Check.That(loginResp).Status(200).Failed() {
		return &spec.ScriptError{
			Type:    "business",
			Code:    "LOGIN_FAIL",
			Message: fmt.Sprintf("登录失败: %s", loginResp.Text()),
			API:     "/api/auth/login",
		}
	}

	token, ok := loginResp.JSON("data.token").(string)
	if !ok || token == "" {
		return fmt.Errorf("响应中未找到 token")
	}

	// 将 token 存入 VU 私有变量，下次迭代可复用（避免每次都登录）
	// 实际压测中可加 token 过期判断逻辑
	ctx.Vars.Set("token", token)

	// Step 2: 查询商品列表（携带 token）
	goodsResp, err := cli.Get(baseURL+"/api/goods?page=1&pageSize=10",
		sdkhttp.WithHeader("Authorization", "Bearer "+token),
		sdkhttp.WithName("/api/goods"),
	)
	if err != nil {
		return err
	}

	ctx.Check.That(goodsResp).Status(200).RTLt(500)

	ctx.Log.Debug("VU[%d] iter[%d] 完成，goods code=%d",
		ctx.VUId, ctx.Iteration, goodsResp.StatusCode)

	return nil
}

// Teardown 所有 VU 结束后执行一次，清理资源。
func (s *httpLoginScript) Teardown(ctx *spec.RunContext, data interface{}) error {
	ctx.Log.Info("Teardown: 压测完成")
	return nil
}
