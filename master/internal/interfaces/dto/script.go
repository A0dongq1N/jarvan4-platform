package dto

// ── 脚本 ──────────────────────────────────────────────────────────────────

type PublishScriptReq struct {
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"        binding:"required,max=128"`
	Description string `json:"description"`
	CommitHash  string `json:"commitHash"  binding:"required"`
	ArtifactURL string `json:"artifactUrl" binding:"required"`
	CommitMsg   string `json:"commitMsg"`
	Author      string `json:"author"`
}

type ScriptResp struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"` // "go" | "python" | "javascript"
	CommitHash  string `json:"commitHash"`
	ArtifactURL string `json:"artifactUrl"`
	CommitMsg   string `json:"commitMsg"`
	Author      string `json:"author"`
	Status      string `json:"status"` // "active" | "offline"
	CreatedAt   string `json:"createdAt"` // ISO 时间字符串
	UpdatedAt   string `json:"updatedAt"` // ISO 时间字符串
}

type ScriptVersionResp struct {
	ID          string `json:"id"`
	CommitHash  string `json:"commitHash"`
	ArtifactURL string `json:"artifactUrl"`
	CommitMsg   string `json:"commitMsg"`
	Author      string `json:"author"`
	CreatedAt   string `json:"createdAt"` // ISO 时间字符串
}
