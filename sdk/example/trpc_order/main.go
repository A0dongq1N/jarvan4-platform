//go:build ignore

// 示例压测脚本：tRPC over HTTP 接口压测
//
// 环境变量：
//   TRPC_ADDR   tRPC 服务地址，如 http://staging.example.com:8080
package main

import (
	"fmt"

	"github.com/Aodongq1n/jarvan4-platform/sdk/spec"
	sdktrpc "github.com/Aodongq1n/jarvan4-platform/sdk/trpc"
)

var Script spec.ScriptEntry = &trpcOrderScript{}

type trpcOrderScript struct{}

type trpcSetupData struct {
	Addr string
}

func (s *trpcOrderScript) Setup(ctx *spec.RunContext) (interface{}, error) {
	addr := ctx.Vars.Env("TRPC_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("TRPC_ADDR 未配置")
	}
	ctx.Log.Info("Setup: tRPC 服务地址 %s", addr)
	return &trpcSetupData{Addr: addr}, nil
}

func (s *trpcOrderScript) Default(ctx *spec.RunContext) error {
	sd := ctx.SetupData.(*trpcSetupData)
	cli := sdktrpc.New(ctx, sd.Addr)

	// 调用订单查询接口
	// POST http://host/trpc.order.OrderService/GetOrder
	var orderResp struct {
		OrderID string  `json:"order_id"`
		Amount  float64 `json:"amount"`
		Status  string  `json:"status"`
	}
	err := cli.Call(ctx,
		"trpc.order.OrderService",
		"GetOrder",
		map[string]interface{}{
			"order_id": fmt.Sprintf("order_%d_%d", ctx.VUId, ctx.Iteration),
		},
		&orderResp,
	)
	if err != nil {
		// ScriptError 会被 Worker 按 Type/Code 聚合到报告错误分析
		return err
	}

	ctx.Log.Debug("VU[%d] order_id=%s status=%s", ctx.VUId, orderResp.OrderID, orderResp.Status)
	return nil
}

func (s *trpcOrderScript) Teardown(ctx *spec.RunContext, data interface{}) error {
	return nil
}
