// Package worker Worker 管理用例服务（应用层）
package worker

import (
	"context"
	"fmt"

	domainWorker "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/worker"
)

// Service Worker 管理用例服务
type Service struct {
	workerRepo domainWorker.WorkerRepo
}

// NewService 构造函数
func NewService(workerRepo domainWorker.WorkerRepo) *Service {
	return &Service{workerRepo: workerRepo}
}

// RegisterWorker Nacos 上线事件触发：注册或恢复 Worker 节点
func (s *Service) RegisterWorker(ctx context.Context, id, addr string, cpuCores int, memGB float64, maxConcurrency int) error {
	w, err := s.workerRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find worker: %w", err)
	}

	if w == nil {
		// 首次注册
		w = domainWorker.NewWorkerNode(id, addr, cpuCores, memGB, maxConcurrency)
	} else {
		// 重新上线：更新地址、状态、节点规格（Nacos metadata 可能已更新）
		w.Rejoin(addr, cpuCores, memGB, maxConcurrency)
	}

	if err := s.workerRepo.Save(ctx, w); err != nil {
		return fmt.Errorf("save worker: %w", err)
	}
	return nil
}

// Heartbeat Worker 心跳上报负载
func (s *Service) Heartbeat(ctx context.Context, id string, cpuUsage, memUsage float64, concurrent int, runningRunID string) error {
	w, err := s.workerRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find worker: %w", err)
	}
	w.UpdateLoad(cpuUsage, memUsage, concurrent, runningRunID)
	if err := s.workerRepo.Save(ctx, w); err != nil {
		return fmt.Errorf("save worker: %w", err)
	}
	return nil
}

// OfflineWorker Nacos 下线事件触发
func (s *Service) OfflineWorker(ctx context.Context, id string) error {
	w, err := s.workerRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find worker: %w", err)
	}
	// 幂等：已下线则直接返回
	if offErr := w.Offline(); offErr != nil {
		return nil
	}
	if err := s.workerRepo.Save(ctx, w); err != nil {
		return fmt.Errorf("save worker: %w", err)
	}
	return nil
}

// ListWorkers 查询所有 Worker 节点
func (s *Service) ListWorkers(ctx context.Context) ([]*domainWorker.WorkerNode, error) {
	return s.workerRepo.ListAll(ctx)
}
