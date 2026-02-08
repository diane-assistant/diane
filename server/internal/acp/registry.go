// Package acp provides a registry client for discovering and installing ACP-compatible agents.
package acp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// RegistryURL is the official ACP agent registry
	RegistryURL = "https://cdn.agentclientprotocol.com/registry/v1/latest/registry.json"
)

// Registry represents the ACP agent registry
type Registry struct {
	Version string          `json:"version"`
	Agents  []RegistryAgent `json:"agents"`
}

// RegistryAgent represents an agent in the registry
type RegistryAgent struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Version      string        `json:"version"`
	Description  string        `json:"description"`
	Repository   string        `json:"repository,omitempty"`
	Authors      []string      `json:"authors,omitempty"`
	License      string        `json:"license,omitempty"`
	Icon         string        `json:"icon,omitempty"`
	Distribution *Distribution `json:"distribution,omitempty"`
}

// Distribution describes how to install/run an agent
type Distribution struct {
	NPX    *NPXDistribution               `json:"npx,omitempty"`
	Binary map[string]*BinaryDistribution `json:"binary,omitempty"`
}

// NPXDistribution describes an npx-based agent
type NPXDistribution struct {
	Package string            `json:"package"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// BinaryDistribution describes a binary download
type BinaryDistribution struct {
	Archive string            `json:"archive"`
	Cmd     string            `json:"cmd"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// RegistryClient fetches and caches the agent registry
type RegistryClient struct {
	CachePath   string
	CacheTTL    time.Duration
	registry    *Registry
	lastFetched time.Time
}

// NewRegistryClient creates a new registry client
func NewRegistryClient() (*RegistryClient, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	cachePath := filepath.Join(home, ".diane", "acp-registry.json")

	return &RegistryClient{
		CachePath: cachePath,
		CacheTTL:  24 * time.Hour, // Cache for 24 hours
	}, nil
}

// GetRegistry returns the agent registry, fetching if needed
func (c *RegistryClient) GetRegistry(forceRefresh bool) (*Registry, error) {
	// Try to load from cache first
	if !forceRefresh && c.registry != nil && time.Since(c.lastFetched) < c.CacheTTL {
		return c.registry, nil
	}

	// Try to load from cache file
	if !forceRefresh {
		if cached, err := c.loadCache(); err == nil {
			c.registry = cached
			c.lastFetched = time.Now()
			return c.registry, nil
		}
	}

	// Fetch from remote
	registry, err := c.fetchRegistry()
	if err != nil {
		// If fetch fails, try to use stale cache
		if cached, cacheErr := c.loadCache(); cacheErr == nil {
			return cached, nil
		}
		return nil, err
	}

	// Save to cache
	c.saveCache(registry)
	c.registry = registry
	c.lastFetched = time.Now()

	return registry, nil
}

// fetchRegistry fetches the registry from the remote URL
func (c *RegistryClient) fetchRegistry() (*Registry, error) {
	resp, err := http.Get(RegistryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry fetch failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry: %w", err)
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	return &registry, nil
}

// loadCache loads the registry from the cache file
func (c *RegistryClient) loadCache() (*Registry, error) {
	data, err := os.ReadFile(c.CachePath)
	if err != nil {
		return nil, err
	}

	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

// saveCache saves the registry to the cache file
func (c *RegistryClient) saveCache(registry *Registry) error {
	dir := filepath.Dir(c.CachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.CachePath, data, 0644)
}

// ListAgents returns all agents from the registry
func (c *RegistryClient) ListAgents() ([]RegistryAgent, error) {
	registry, err := c.GetRegistry(false)
	if err != nil {
		return nil, err
	}
	return registry.Agents, nil
}

// GetAgent returns a specific agent by ID
func (c *RegistryClient) GetAgent(id string) (*RegistryAgent, error) {
	registry, err := c.GetRegistry(false)
	if err != nil {
		return nil, err
	}

	for _, agent := range registry.Agents {
		if agent.ID == id {
			return &agent, nil
		}
	}

	return nil, fmt.Errorf("agent '%s' not found in registry", id)
}

// GetPlatformKey returns the platform key for the current system
func GetPlatformKey() string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map Go arch to registry arch names
	archMap := map[string]string{
		"amd64": "x86_64",
		"arm64": "aarch64",
	}

	if mapped, ok := archMap[arch]; ok {
		arch = mapped
	}

	return fmt.Sprintf("%s-%s", os, arch)
}

// GetInstallCommand returns the command to install/run an agent
func (a *RegistryAgent) GetInstallCommand() (cmd string, args []string, env map[string]string, err error) {
	if a.Distribution == nil {
		return "", nil, nil, fmt.Errorf("agent '%s' has no distribution info", a.ID)
	}

	// Prefer npx for Node.js agents
	if a.Distribution.NPX != nil {
		npx := a.Distribution.NPX
		return "npx", append([]string{npx.Package}, npx.Args...), npx.Env, nil
	}

	// Check for binary distribution
	if a.Distribution.Binary != nil {
		platform := GetPlatformKey()
		if bin, ok := a.Distribution.Binary[platform]; ok {
			// For binary, we'd need to download and extract first
			// Return the command assuming the binary is installed
			return bin.Cmd, bin.Args, bin.Env, nil
		}
		return "", nil, nil, fmt.Errorf("no binary available for platform %s", platform)
	}

	return "", nil, nil, fmt.Errorf("no supported distribution method for agent '%s'", a.ID)
}

// IsNPXBased returns true if the agent uses npx
func (a *RegistryAgent) IsNPXBased() bool {
	return a.Distribution != nil && a.Distribution.NPX != nil
}

// IsBinaryBased returns true if the agent uses binary distribution
func (a *RegistryAgent) IsBinaryBased() bool {
	return a.Distribution != nil && a.Distribution.Binary != nil
}

// CheckInstalled checks if the agent is available on the system
func (a *RegistryAgent) CheckInstalled() bool {
	if a.Distribution == nil {
		return false
	}

	if a.Distribution.NPX != nil {
		// Check if npx is available
		_, err := exec.LookPath("npx")
		return err == nil
	}

	if a.Distribution.Binary != nil {
		platform := GetPlatformKey()
		if bin, ok := a.Distribution.Binary[platform]; ok {
			cmd := strings.TrimPrefix(bin.Cmd, "./")
			_, err := exec.LookPath(cmd)
			return err == nil
		}
	}

	return false
}

// GalleryEntry represents a pre-configured agent for the gallery
type GalleryEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon,omitempty"`
	Category    string   `json:"category"`     // "coding", "general", "specialized"
	Provider    string   `json:"provider"`     // "anthropic", "google", "openai", etc.
	InstallType string   `json:"install_type"` // "npx", "binary", "manual"
	Tags        []string `json:"tags,omitempty"`
	Featured    bool     `json:"featured"`
}

// Gallery provides a curated list of agents with easy installation
type Gallery struct {
	registryClient *RegistryClient
	entries        []GalleryEntry
}

// NewGallery creates a new agent gallery
func NewGallery() (*Gallery, error) {
	client, err := NewRegistryClient()
	if err != nil {
		return nil, err
	}

	g := &Gallery{
		registryClient: client,
		entries:        getBuiltInGalleryEntries(),
	}

	return g, nil
}

// getBuiltInGalleryEntries returns pre-configured gallery entries
func getBuiltInGalleryEntries() []GalleryEntry {
	return []GalleryEntry{
		{
			ID:          "claude-code-acp",
			Name:        "Claude Code",
			Description: "Anthropic's Claude AI coding assistant",
			Category:    "coding",
			Provider:    "anthropic",
			InstallType: "npx",
			Tags:        []string{"ai", "coding", "anthropic", "claude"},
			Featured:    true,
		},
		{
			ID:          "gemini",
			Name:        "Gemini CLI",
			Description: "Google's Gemini AI assistant",
			Category:    "coding",
			Provider:    "google",
			InstallType: "npx",
			Tags:        []string{"ai", "coding", "google", "gemini"},
			Featured:    true,
		},
		{
			ID:          "github-copilot",
			Name:        "GitHub Copilot",
			Description: "GitHub's AI pair programmer",
			Category:    "coding",
			Provider:    "microsoft",
			InstallType: "npx",
			Tags:        []string{"ai", "coding", "github", "copilot"},
			Featured:    true,
		},
		{
			ID:          "opencode",
			Name:        "OpenCode",
			Description: "Open-source AI coding agent",
			Category:    "coding",
			Provider:    "sst",
			InstallType: "npx",
			Tags:        []string{"ai", "coding", "opensource"},
			Featured:    true,
		},
		{
			ID:          "codex-acp",
			Name:        "Codex CLI",
			Description: "OpenAI's coding assistant",
			Category:    "coding",
			Provider:    "openai",
			InstallType: "binary",
			Tags:        []string{"ai", "coding", "openai"},
			Featured:    true,
		},
		{
			ID:          "auggie",
			Name:        "Auggie CLI",
			Description: "Augment Code's software agent",
			Category:    "coding",
			Provider:    "augment",
			InstallType: "npx",
			Tags:        []string{"ai", "coding"},
			Featured:    false,
		},
		{
			ID:          "qwen-code",
			Name:        "Qwen Code",
			Description: "Alibaba's Qwen coding assistant",
			Category:    "coding",
			Provider:    "alibaba",
			InstallType: "npx",
			Tags:        []string{"ai", "coding", "qwen"},
			Featured:    false,
		},
		{
			ID:          "kimi",
			Name:        "Kimi CLI",
			Description: "Moonshot AI's Kimi assistant",
			Category:    "coding",
			Provider:    "moonshot",
			InstallType: "npx",
			Tags:        []string{"ai", "coding", "kimi"},
			Featured:    false,
		},
		{
			ID:          "mistral-vibe",
			Name:        "Mistral Vibe",
			Description: "Mistral AI's coding assistant",
			Category:    "coding",
			Provider:    "mistral",
			InstallType: "binary",
			Tags:        []string{"ai", "coding", "mistral"},
			Featured:    false,
		},
	}
}

// ListGallery returns all gallery entries with registry info
func (g *Gallery) ListGallery() ([]GalleryEntry, error) {
	// Sync with registry to get latest info
	agents, err := g.registryClient.ListAgents()
	if err != nil {
		// Return built-in entries if registry is unavailable
		return g.entries, nil
	}

	// Create a map for quick lookup
	registryMap := make(map[string]*RegistryAgent)
	for i := range agents {
		registryMap[agents[i].ID] = &agents[i]
	}

	// Update entries with registry info
	result := make([]GalleryEntry, 0, len(g.entries))
	for _, entry := range g.entries {
		if regAgent, ok := registryMap[entry.ID]; ok {
			entry.Icon = regAgent.Icon
			entry.Description = regAgent.Description
		}
		result = append(result, entry)
	}

	return result, nil
}

// ListFeatured returns featured agents
func (g *Gallery) ListFeatured() ([]GalleryEntry, error) {
	entries, err := g.ListGallery()
	if err != nil {
		return nil, err
	}

	featured := make([]GalleryEntry, 0)
	for _, e := range entries {
		if e.Featured {
			featured = append(featured, e)
		}
	}

	return featured, nil
}

// GetInstallInfo returns installation information for an agent
func (g *Gallery) GetInstallInfo(id string) (*InstallInfo, error) {
	agent, err := g.registryClient.GetAgent(id)
	if err != nil {
		return nil, err
	}

	info := &InstallInfo{
		ID:          agent.ID,
		Name:        agent.Name,
		Version:     agent.Version,
		Description: agent.Description,
		Repository:  agent.Repository,
		WorkDirArg:  GetWorkDirArg(id),
	}

	cmd, args, env, err := agent.GetInstallCommand()
	if err != nil {
		info.Error = err.Error()
	} else {
		info.Command = cmd
		info.Args = args
		info.Env = env
		info.Available = agent.CheckInstalled()
	}

	if agent.IsNPXBased() {
		info.InstallType = "npx"
		info.InstallCmd = fmt.Sprintf("npx %s", agent.Distribution.NPX.Package)
	} else if agent.IsBinaryBased() {
		info.InstallType = "binary"
		platform := GetPlatformKey()
		if bin, ok := agent.Distribution.Binary[platform]; ok {
			info.DownloadURL = bin.Archive
		}
	}

	return info, nil
}

// InstallInfo contains installation details for an agent
type InstallInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Repository  string            `json:"repository,omitempty"`
	InstallType string            `json:"install_type"`
	InstallCmd  string            `json:"install_cmd,omitempty"`
	DownloadURL string            `json:"download_url,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Available   bool              `json:"available"`
	Error       string            `json:"error,omitempty"`
	// Workspace-specific fields
	WorkDir    string `json:"workdir,omitempty"`     // Suggested working directory
	WorkDirArg string `json:"workdir_arg,omitempty"` // CLI arg for setting workdir (e.g., "--cwd", "--include-directories")
}

// RefreshRegistry forces a refresh of the registry cache
func (g *Gallery) RefreshRegistry() error {
	_, err := g.registryClient.GetRegistry(true)
	return err
}

// WorkDirArgs maps agent IDs to their CLI argument for setting the working directory
var WorkDirArgs = map[string]string{
	"opencode":        "--cwd",
	"gemini":          "--include-directories",
	"claude-code-acp": "--cwd",
	"github-copilot":  "--workspace-folder",
	"codex-acp":       "--cwd",
	"auggie":          "--cwd",
	"qwen-code":       "--cwd",
	"kimi":            "--cwd",
	"mistral-vibe":    "--cwd",
}

// GetWorkDirArg returns the CLI argument for setting the working directory for an agent
func GetWorkDirArg(agentID string) string {
	if arg, ok := WorkDirArgs[agentID]; ok {
		return arg
	}
	return "--cwd" // Default
}

// BuildArgsWithWorkDir returns the full args slice including the workdir argument
func BuildArgsWithWorkDir(agentID string, baseArgs []string, workDir string) []string {
	if workDir == "" {
		return baseArgs
	}

	workDirArg := GetWorkDirArg(agentID)

	// Check if workdir arg is already present
	for _, arg := range baseArgs {
		if arg == workDirArg {
			return baseArgs // Already has workdir
		}
	}

	return append(baseArgs, workDirArg, workDir)
}
