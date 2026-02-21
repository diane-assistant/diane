package store

// WorkspaceConfig represents the workspace configuration for emergent agents
type WorkspaceConfig struct {
	BaseImage     string   `json:"base_image,omitempty"`
	RepoURL       string   `json:"repo_url,omitempty"`
	RepoBranch    string   `json:"repo_branch,omitempty"`
	Provider      string   `json:"provider,omitempty"`
	SetupCommands []string `json:"setup_commands,omitempty"`
}
