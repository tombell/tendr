package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadStrictValidConfigAndResolveInheritedPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "tendr")
	path := writeConfig(t, configDir, "demo.yml", `
root: ~/Code/demo
before_start: ["echo project"]
after_start: ["echo ready"]
after_stop: ["echo stopped"]
workspaces:
  - label: api
    root: services/api
    before_start: ["echo before api"]
    after_start: ["echo after api"]
    tabs:
      - label: server
        root: cmd/server
        commands: ["go run ."]
        panes:
          - direction: right
            ratio: 0.4
            root: ../../tests
            commands: ["go test ./..."]
      - label: logs
        commands: ["tail -f app.log"]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantProjectRoot := filepath.Join(home, "Code", "demo")
	if cfg.Root != wantProjectRoot {
		t.Errorf("Root = %q, want %q", cfg.Root, wantProjectRoot)
	}
	if got, want := cfg.AfterStart, []string{"echo ready"}; !slices.Equal(got, want) {
		t.Errorf("AfterStart = %q, want %q", got, want)
	}
	workspace := cfg.Workspaces[0]
	wantWorkspaceRoot := filepath.Join(wantProjectRoot, "services", "api")
	if workspace.Root != wantWorkspaceRoot {
		t.Errorf("workspace root = %q, want %q", workspace.Root, wantWorkspaceRoot)
	}
	if got, want := workspace.Tabs[0].Root, filepath.Join(wantWorkspaceRoot, "cmd", "server"); got != want {
		t.Errorf("tab root = %q, want %q", got, want)
	}
	if got, want := workspace.Tabs[0].Panes[0].Root, filepath.Join(wantWorkspaceRoot, "tests"); got != want {
		t.Errorf("pane root = %q, want %q", got, want)
	}
	if got := workspace.Tabs[1].Root; got != wantWorkspaceRoot {
		t.Errorf("inherited tab root = %q, want %q", got, wantWorkspaceRoot)
	}
}

func TestLoadRejectsUnknownAndLegacyFields(t *testing.T) {
	tests := map[string]string{
		"unknown": `root: /tmp
workspaces:
  - label: api
    tabs:
      - label: shell
        mystery: true
`,
		"tm vocabulary": `root: /tmp
sessions:
  - name: api
    windows: []
`,
	}

	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := writeConfig(t, t.TempDir(), "invalid.yml", contents)
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), "field") {
				t.Fatalf("Load() error = %v, want unknown field error", err)
			}
		})
	}
}

func TestValidateReportsAllTopologyProblems(t *testing.T) {
	ratio := 1.2
	cfg := Config{
		AfterStart: []string{" "},
		Workspaces: []Workspace{
			{
				Label: "api",
				Tabs: []Tab{
					{Label: "shell", Commands: []string{""}, Panes: []Pane{{Direction: "left", Ratio: &ratio}}},
					{Label: "shell"},
				},
			},
			{Label: "api"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	for _, want := range []string{
		"root is required",
		"after_start[0] must not be empty",
		"commands[0] must not be empty",
		"direction must be right or down",
		"ratio must be greater than 0 and less than 1",
		"tabs[1].label must be unique",
		"workspaces[1].label must be unique",
		"workspaces[1].tabs must contain at least one tab",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() error = %q, want substring %q", err, want)
		}
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	path := writeConfig(t, t.TempDir(), "multiple.yml", `
root: /tmp
workspaces:
  - label: api
    tabs:
      - label: shell
---
root: /other
workspaces: []
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("Load() error = %v, want multiple document error", err)
	}
}

func TestLoadRejectsInvalidYAML(t *testing.T) {
	path := writeConfig(t, t.TempDir(), "invalid.yml", "root: [")
	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil")
	}
}

func writeConfig(t *testing.T, dir, name, contents string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
