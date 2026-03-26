package kiro

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sleuth-io/sx/internal/bootstrap"
	"github.com/sleuth-io/sx/internal/clients/kiro/handlers"
	"github.com/sleuth-io/sx/internal/logger"
)

// kiroHookFile represents the JSON structure of a .kiro.hook file
type kiroHookFile struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	When        kiroHookWhen `json:"when"`
	Then        kiroHookThen `json:"then"`
}

// kiroHookWhen represents the trigger condition for a hook
type kiroHookWhen struct {
	Type string `json:"type"`
}

// kiroHookThen represents the action to take when a hook fires
type kiroHookThen struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

const (
	hookFileInstall     = "sx-install.kiro.hook"
	hookFileReportUsage = "sx-report-usage.kiro.hook"
)

// installKiroHooks installs hook files to {repoRoot}/.kiro/hooks/
func installKiroHooks(repoRoot string, opts []bootstrap.Option) error {
	hooksDir := filepath.Join(repoRoot, handlers.ConfigDir, handlers.DirHooks)
	log := logger.Get()

	// Ensure hooks directory exists
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	modified := false

	// Install promptSubmit hook for auto-update (if enabled)
	if bootstrap.ContainsKey(opts, bootstrap.SessionHookKey) {
		hookPath := filepath.Join(hooksDir, hookFileInstall)
		if !hookFileHasCommand(hookPath, "sx install") {
			hook := kiroHookFile{
				Name:        "sx install",
				Description: "Auto-update sx assets when a session starts",
				Version:     "1",
				When:        kiroHookWhen{Type: "sessionStart"},
				Then: kiroHookThen{
					Type:    "runCommand",
					Command: "sx install --hook-mode --client=kiro",
				},
			}
			if err := writeKiroHookFile(hookPath, hook); err != nil {
				return err
			}
			log.Info("hook installed", "hook", "promptSubmit", "file", hookFileInstall)
			modified = true
		}
	}

	// Install postToolUse hook for usage tracking (if enabled)
	if bootstrap.ContainsKey(opts, bootstrap.AnalyticsHookKey) {
		hookPath := filepath.Join(hooksDir, hookFileReportUsage)
		if !hookFileHasCommand(hookPath, "sx report-usage") {
			hook := kiroHookFile{
				Name:        "sx report-usage",
				Description: "Track skill usage for analytics",
				Version:     "1",
				When:        kiroHookWhen{Type: "postToolUse"},
				Then: kiroHookThen{
					Type:    "runCommand",
					Command: "sx report-usage --client=kiro",
				},
			}
			if err := writeKiroHookFile(hookPath, hook); err != nil {
				return err
			}
			log.Info("hook installed", "hook", "postToolUse", "file", hookFileReportUsage)
			modified = true
		}
	}

	if modified {
		fmt.Printf("  Installed Kiro hooks to %s\n", hooksDir)
	}

	return nil
}

// uninstallKiroHooks removes sx hook files from {repoRoot}/.kiro/hooks/
func uninstallKiroHooks(repoRoot string, uninstallSession, uninstallAnalytics bool) error {
	hooksDir := filepath.Join(repoRoot, handlers.ConfigDir, handlers.DirHooks)
	log := logger.Get()

	if uninstallSession {
		hookPath := filepath.Join(hooksDir, hookFileInstall)
		if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", hookFileInstall, err)
		}
		log.Info("hook removed", "file", hookFileInstall)
	}

	if uninstallAnalytics {
		hookPath := filepath.Join(hooksDir, hookFileReportUsage)
		if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove %s: %w", hookFileReportUsage, err)
		}
		log.Info("hook removed", "file", hookFileReportUsage)
	}

	return nil
}

// writeKiroHookFile writes a .kiro.hook JSON file
func writeKiroHookFile(path string, hook kiroHookFile) error {
	data, err := json.MarshalIndent(hook, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hook file: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write hook file %s: %w", filepath.Base(path), err)
	}
	return nil
}

// hookFileHasCommand checks if a hook file exists and already contains the expected command prefix
func hookFileHasCommand(path string, commandPrefix string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var hook kiroHookFile
	if err := json.Unmarshal(data, &hook); err != nil {
		return false
	}

	return strings.HasPrefix(hook.Then.Command, commandPrefix)
}

// findGitRoot finds the root of the git repository from the current working directory.
// Returns empty string if not in a git repository.
func findGitRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	dir := cwd
	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
