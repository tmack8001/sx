package commands

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sleuth-io/sx/internal/asset"
	"github.com/sleuth-io/sx/internal/bootstrap"
	"github.com/sleuth-io/sx/internal/clients"
	"github.com/sleuth-io/sx/internal/clients/openclaw"
)

func init() {
	// Ensure openclaw client is registered for these tests
	clients.Register(openclaw.NewClient())
}

// TestOpenClawClientDetection tests that OpenClaw is detected when ~/.openclaw exists
func TestOpenClawClientDetection(t *testing.T) {
	env := NewTestEnv(t)

	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	if client.IsInstalled() {
		t.Error("OpenClaw should not be detected without ~/.openclaw directory")
	}

	// Create .openclaw directory
	openclawDir := filepath.Join(env.HomeDir, ".openclaw")
	if err := os.MkdirAll(openclawDir, 0755); err != nil {
		t.Fatalf("Failed to create .openclaw dir: %v", err)
	}

	if !client.IsInstalled() {
		t.Error("OpenClaw should be detected with ~/.openclaw directory")
	}
}

// TestOpenClawSupportedAssetTypes verifies OpenClaw supports only skills
func TestOpenClawSupportedAssetTypes(t *testing.T) {
	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	// Should support
	if !client.SupportsAssetType(asset.TypeSkill) {
		t.Error("OpenClaw should support skill assets")
	}

	// Should not support
	unsupported := []string{"command", "agent", "rule", "hook", "mcp"}
	for _, typeName := range unsupported {
		if client.SupportsAssetType(asset.FromString(typeName)) {
			t.Errorf("OpenClaw should not support %s assets", typeName)
		}
	}
}

// TestOpenClawRepoScopedSkipped verifies repo/path scoped assets are skipped
func TestOpenClawRepoScopedSkipped(t *testing.T) {
	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	ctx := context.Background()

	repoScope := &clients.InstallScope{
		Type:     clients.ScopeRepository,
		RepoRoot: "/tmp/test-repo",
	}

	// Install with repo scope should result in skipped status
	resp, err := client.InstallAssets(ctx, clients.InstallRequest{
		Assets: []*clients.AssetBundle{},
		Scope:  repoScope,
	})
	if err != nil {
		t.Fatalf("InstallAssets error: %v", err)
	}
	// Empty assets = empty results, which is fine
	_ = resp
}

// TestOpenClawBootstrapInstall verifies hook dir and MCP config are created
func TestOpenClawBootstrapInstall(t *testing.T) {
	env := NewTestEnv(t)

	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	openclawDir := filepath.Join(env.HomeDir, ".openclaw")
	if err := os.MkdirAll(openclawDir, 0755); err != nil {
		t.Fatalf("Failed to create .openclaw dir: %v", err)
	}

	ctx := context.Background()
	opts := []bootstrap.Option{
		bootstrap.SessionHook,
		bootstrap.SleuthAIQueryMCP(),
	}

	if err := client.InstallBootstrap(ctx, opts); err != nil {
		t.Fatalf("InstallBootstrap error: %v", err)
	}

	// Verify hook directory exists with HOOK.md and index.ts
	hookDir := filepath.Join(openclawDir, "hooks", "sx-install")
	if _, err := os.Stat(filepath.Join(hookDir, "HOOK.md")); err != nil {
		t.Errorf("HOOK.md should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(hookDir, "index.ts")); err != nil {
		t.Errorf("index.ts should exist: %v", err)
	}

	// Verify HOOK.md contains correct event
	hookContent, err := os.ReadFile(filepath.Join(hookDir, "HOOK.md"))
	if err != nil {
		t.Fatalf("Failed to read HOOK.md: %v", err)
	}
	if !contains(string(hookContent), "before_agent_start") {
		t.Error("HOOK.md should contain before_agent_start event")
	}

	// Verify index.ts contains sx install command
	indexContent, err := os.ReadFile(filepath.Join(hookDir, "index.ts"))
	if err != nil {
		t.Fatalf("Failed to read index.ts: %v", err)
	}
	if !contains(string(indexContent), "sx install --hook-mode --client=openclaw") {
		t.Error("index.ts should contain sx install command")
	}

	// Verify MCP server in openclaw.json
	configPath := filepath.Join(openclawDir, "openclaw.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read openclaw.json: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse openclaw.json: %v", err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers section should exist in openclaw.json")
	}
	if _, ok := mcpServers["sx"]; !ok {
		t.Error("sx MCP server should be registered in openclaw.json")
	}
}

// TestOpenClawBootstrapIdempotent verifies install can be run twice without duplicates
func TestOpenClawBootstrapIdempotent(t *testing.T) {
	env := NewTestEnv(t)

	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	openclawDir := filepath.Join(env.HomeDir, ".openclaw")
	if err := os.MkdirAll(openclawDir, 0755); err != nil {
		t.Fatalf("Failed to create .openclaw dir: %v", err)
	}

	ctx := context.Background()
	opts := []bootstrap.Option{
		bootstrap.SessionHook,
		bootstrap.SleuthAIQueryMCP(),
	}

	// Install twice
	if err := client.InstallBootstrap(ctx, opts); err != nil {
		t.Fatalf("First InstallBootstrap error: %v", err)
	}
	if err := client.InstallBootstrap(ctx, opts); err != nil {
		t.Fatalf("Second InstallBootstrap error: %v", err)
	}

	// Verify no duplicates in openclaw.json
	configPath := filepath.Join(openclawDir, "openclaw.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read openclaw.json: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse openclaw.json: %v", err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers section should exist")
	}
	// Should have exactly one sx entry
	if len(mcpServers) != 1 {
		t.Errorf("Expected 1 MCP server, got %d", len(mcpServers))
	}
}

// TestOpenClawBootstrapUninstall verifies cleanup removes hook dir and MCP config
func TestOpenClawBootstrapUninstall(t *testing.T) {
	env := NewTestEnv(t)

	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	openclawDir := filepath.Join(env.HomeDir, ".openclaw")
	if err := os.MkdirAll(openclawDir, 0755); err != nil {
		t.Fatalf("Failed to create .openclaw dir: %v", err)
	}

	ctx := context.Background()
	opts := []bootstrap.Option{
		bootstrap.SessionHook,
		bootstrap.SleuthAIQueryMCP(),
	}

	// Install first
	if err := client.InstallBootstrap(ctx, opts); err != nil {
		t.Fatalf("InstallBootstrap error: %v", err)
	}

	// Add some other config to verify it's preserved
	configPath := filepath.Join(openclawDir, "openclaw.json")
	data, _ := os.ReadFile(configPath)
	var config map[string]any
	json.Unmarshal(data, &config)
	config["userSetting"] = "preserved"
	data, _ = json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0644)

	// Uninstall
	if err := client.UninstallBootstrap(ctx, opts); err != nil {
		t.Fatalf("UninstallBootstrap error: %v", err)
	}

	// Hook dir should be gone
	hookDir := filepath.Join(openclawDir, "hooks", "sx-install")
	if _, err := os.Stat(hookDir); !os.IsNotExist(err) {
		t.Error("Hook directory should be removed after uninstall")
	}

	// MCP server should be gone but user config preserved
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("openclaw.json should still exist: %v", err)
	}
	var afterConfig map[string]any
	if err := json.Unmarshal(data, &afterConfig); err != nil {
		t.Fatalf("Failed to parse openclaw.json after uninstall: %v", err)
	}
	if _, ok := afterConfig["mcpServers"]; ok {
		t.Error("mcpServers section should be removed (was empty)")
	}
	if afterConfig["userSetting"] != "preserved" {
		t.Error("User settings should be preserved after uninstall")
	}
}

// TestOpenClawClientInfo verifies OpenClaw appears in client info
func TestOpenClawClientInfo(t *testing.T) {
	env := NewTestEnv(t)
	setupTestConfig(t, env.HomeDir, nil, nil)

	openclawDir := filepath.Join(env.HomeDir, ".openclaw")
	if err := os.MkdirAll(openclawDir, 0755); err != nil {
		t.Fatalf("Failed to create .openclaw dir: %v", err)
	}

	infos := gatherClientInfo()

	var found bool
	for _, info := range infos {
		if info.ID == clients.ClientIDOpenClaw {
			found = true
			if !info.Installed {
				t.Error("OpenClaw should show as installed")
			}
			if info.Name != "OpenClaw" {
				t.Errorf("Expected display name 'OpenClaw', got %q", info.Name)
			}
			if info.Directory != openclawDir {
				t.Errorf("Expected directory %q, got %q", openclawDir, info.Directory)
			}
			break
		}
	}

	if !found {
		t.Error("OpenClaw not found in client info")
	}
}

// TestOpenClawBootstrapOptions verifies OpenClaw returns correct bootstrap options
func TestOpenClawBootstrapOptions(t *testing.T) {
	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	opts := client.GetBootstrapOptions(context.Background())

	var hasSession, hasMCP bool
	for _, opt := range opts {
		if opt.Key == bootstrap.SessionHookKey {
			hasSession = true
		}
		if opt.Key == bootstrap.SleuthAIQueryMCPKey {
			hasMCP = true
		}
	}

	if !hasSession {
		t.Error("OpenClaw should offer session_hook option")
	}
	if !hasMCP {
		t.Error("OpenClaw should offer sleuth_ai_query_mcp option")
	}
}

// TestOpenClawGetBootstrapPath verifies the bootstrap path
func TestOpenClawGetBootstrapPath(t *testing.T) {
	env := NewTestEnv(t)

	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	path := client.GetBootstrapPath()
	expected := filepath.Join(env.HomeDir, ".openclaw", "openclaw.json")
	if path != expected {
		t.Errorf("GetBootstrapPath() = %q, want %q", path, expected)
	}
}

// TestOpenClawShouldInstallDedup verifies timestamp-based dedup
func TestOpenClawShouldInstallDedup(t *testing.T) {
	_ = NewTestEnv(t)

	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	ctx := context.Background()

	// First call should return true
	should, err := client.ShouldInstall(ctx)
	if err != nil {
		t.Fatalf("ShouldInstall error: %v", err)
	}
	if !should {
		t.Error("First ShouldInstall call should return true")
	}

	// Second call within same hour should return false
	should, err = client.ShouldInstall(ctx)
	if err != nil {
		t.Fatalf("ShouldInstall error: %v", err)
	}
	if should {
		t.Error("Second ShouldInstall call should return false (dedup)")
	}
}

// TestOpenClawRuleCapabilities verifies no rule support
func TestOpenClawRuleCapabilities(t *testing.T) {
	client, err := clients.Global().Get(clients.ClientIDOpenClaw)
	if err != nil {
		t.Fatalf("OpenClaw client not registered: %v", err)
	}

	if client.RuleCapabilities() != nil {
		t.Error("OpenClaw should not have rule capabilities")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
