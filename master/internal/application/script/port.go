package script

import (
	"context"

	domainScript "github.com/Aodongq1n/jarvan4-platform/master/internal/domain/script"
)

// ScriptUseCase 脚本管理入站端口
type ScriptUseCase interface {
	PublishScript(ctx context.Context, projectID, name, description, commitHash, artifactURL, commitMsg, author string) (*domainScript.Script, error)
	GetScript(ctx context.Context, id string) (*domainScript.Script, error)
	ListScripts(ctx context.Context, projectID string, page, pageSize int) ([]*domainScript.Script, int64, error)
	ListVersions(ctx context.Context, scriptID string, page, pageSize int) ([]*domainScript.ScriptVersion, int64, error)
	OfflineScript(ctx context.Context, id, operatedBy string) error
}

var _ ScriptUseCase = (*Service)(nil)
