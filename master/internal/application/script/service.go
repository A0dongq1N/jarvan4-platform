// Package script 脚本用例服务（应用层）
package script

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domainScript "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/script"
	"github.com/Aodongq1n/jarvan4-platform/master/internal/domain"
)

// Service 脚本用例服务
type Service struct {
	scriptRepo  domainScript.ScriptRepo
	versionRepo domainScript.ScriptVersionRepo
}

// NewService 构造函数
func NewService(scriptRepo domainScript.ScriptRepo, versionRepo domainScript.ScriptVersionRepo) *Service {
	return &Service{scriptRepo: scriptRepo, versionRepo: versionRepo}
}

// PublishScript CI 回调：注册或更新脚本及版本
// 若同名脚本已存在则更新（重新上线并更新 commitHash），否则新建
func (s *Service) PublishScript(ctx context.Context, projectID, name, description, commitHash, artifactURL, commitMsg, author string) (*domainScript.Script, error) {
	sc, err := s.scriptRepo.FindByName(ctx, projectID, name)
	if err != nil && err != domain.ErrNotFound {
		return nil, fmt.Errorf("find script by name: %w", err)
	}

	if err == domain.ErrNotFound {
		// 首次发布：创建新脚本
		sc = domainScript.NewScript(projectID, name, description, "go", author)
		sc.ID = uuid.NewString()
	}

	// 更新到最新版本（Publish 会将 status 重置为 1）
	sc.Publish(commitHash, artifactURL, commitMsg, author)

	if err := s.scriptRepo.Save(ctx, sc); err != nil {
		return nil, fmt.Errorf("save script: %w", err)
	}

	// 追加版本历史记录
	version := &domainScript.ScriptVersion{
		ID:          uuid.NewString(),
		ScriptID:    sc.ID,
		CommitHash:  commitHash,
		ArtifactURL: artifactURL,
		CommitMsg:   commitMsg,
		Author:      author,
	}
	if err := s.versionRepo.Save(ctx, version); err != nil {
		// 版本历史写失败不影响主流程，记录错误即可
		return sc, fmt.Errorf("save script version: %w", err)
	}

	return sc, nil
}

// GetScript 查询脚本详情
func (s *Service) GetScript(ctx context.Context, id string) (*domainScript.Script, error) {
	return s.scriptRepo.FindByID(ctx, id)
}

// ListScripts 分页查询项目下脚本列表
func (s *Service) ListScripts(ctx context.Context, projectID string, page, pageSize int) ([]*domainScript.Script, int64, error) {
	return s.scriptRepo.ListByProjectID(ctx, projectID, page, pageSize)
}

// ListVersions 查询脚本版本历史
func (s *Service) ListVersions(ctx context.Context, scriptID string, page, pageSize int) ([]*domainScript.ScriptVersion, int64, error) {
	return s.versionRepo.ListByScriptID(ctx, scriptID, page, pageSize)
}

// OfflineScript 下线脚本
func (s *Service) OfflineScript(ctx context.Context, id, operatedBy string) error {
	sc, err := s.scriptRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("find script: %w", err)
	}
	if err := sc.Offline(); err != nil {
		return fmt.Errorf("offline script: %w", err)
	}
	if err := s.scriptRepo.Save(ctx, sc); err != nil {
		return fmt.Errorf("save script: %w", err)
	}
	return nil
}
