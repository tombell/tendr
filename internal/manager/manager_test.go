package manager

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/tombell/tendr/internal/config"
	"github.com/tombell/tendr/internal/herdr"
)

func TestStartOrdersHooksTopologyCommandsAndFocus(t *testing.T) {
	events := []string{}
	sessions := []string{}
	client := &fakeHerdr{events: &events}
	shell := fakeShell{events: &events, sessions: &sessions}
	manager := New(client, shell)
	ratio := 0.35
	cfg := &config.Config{
		Root:        "/project",
		BeforeStart: []string{"project hook"},
		AfterStart:  []string{"project ready"},
		Workspaces: []config.Workspace{
			{
				Label:       "api",
				Root:        "/project/api",
				BeforeStart: []string{"workspace before"},
				AfterStart:  []string{"workspace after"},
				Tabs: []config.Tab{
					{
						Label:    "server",
						Root:     "/project/api/server",
						Commands: []string{"go run ."},
						Panes: []config.Pane{
							{Direction: "right", Ratio: &ratio, Root: "/project/api/tests", Commands: []string{"go test ./..."}},
							{Direction: "down", Root: "/project/api/logs", Commands: []string{"tail -f app.log"}},
						},
					},
					{Label: "editor", Root: "/project/api", Commands: []string{"nvim ."}},
				},
			},
		},
	}

	if err := manager.Start(context.Background(), "demo", cfg); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	want := []string{
		"exists demo",
		"shell /project project hook",
		"start demo",
		"shell /project/api workspace before",
		"workspace demo api /project/api/server",
		"rename-tab demo tab-1 server",
		"run demo pane-1 go run .",
		"split demo pane-1 right /project/api/tests 0.35",
		"run demo pane-split-1 go test ./...",
		"split demo pane-1 down /project/api/logs none",
		"run demo pane-split-2 tail -f app.log",
		"tab demo workspace-1 editor /project/api",
		"run demo pane-tab-2 nvim .",
		"focus demo workspace-1",
		"shell /project/api workspace after",
		"shell /project project ready",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(events, "\n"), strings.Join(want, "\n"))
	}
	if want := []string{"demo", "demo", "demo", "demo"}; !reflect.DeepEqual(sessions, want) {
		t.Fatalf("hook sessions = %#v, want %#v", sessions, want)
	}
}

func TestStartReportsProjectAfterStartFailureAfterCreation(t *testing.T) {
	events := []string{}
	manager := New(&fakeHerdr{events: &events}, fakeShell{events: &events, failAt: "shell /project ready"})
	cfg := validConfig()
	cfg.AfterStart = []string{"ready"}

	err := manager.Start(context.Background(), "demo", cfg)
	if err == nil || !strings.Contains(err.Error(), "project after_start") {
		t.Fatalf("Start() error = %v", err)
	}
	if !containsEvent(events, "workspace demo") || !containsEvent(events, "shell /project ready") {
		t.Fatalf("after_start did not run after workspace creation: %#v", events)
	}
}

func TestStartIsIdempotentForExistingSession(t *testing.T) {
	events := []string{}
	client := &fakeHerdr{events: &events, exists: true}
	manager := New(client, fakeShell{events: &events})
	cfg := validConfig()

	if err := manager.Start(context.Background(), "demo", cfg); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if want := []string{"exists demo"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestStartPreservesPartialSessionAfterFailure(t *testing.T) {
	events := []string{}
	client := &fakeHerdr{events: &events, failAt: "run demo pane-1 serve"}
	manager := New(client, fakeShell{events: &events})
	cfg := validConfig()
	cfg.Workspaces[0].Tabs[0].Commands = []string{"serve", "never"}

	err := manager.Start(context.Background(), "demo", cfg)
	if err == nil || !strings.Contains(err.Error(), "populate tab") {
		t.Fatalf("Start() error = %v", err)
	}
	if containsEvent(events, "delete") {
		t.Fatalf("unexpected rollback: %#v", events)
	}
	if containsEvent(events, "never") || containsEvent(events, "workspace after") {
		t.Fatalf("work continued after failure: %#v", events)
	}
	if !containsEvent(events, "start demo") || !containsEvent(events, "workspace demo") {
		t.Fatalf("failure did not happen after partial creation: %#v", events)
	}
}

func TestStartStopsAtFailingProjectHook(t *testing.T) {
	events := []string{}
	manager := New(&fakeHerdr{events: &events}, fakeShell{events: &events, failAt: "shell /project project hook"})
	cfg := validConfig()
	cfg.BeforeStart = []string{"project hook"}

	if err := manager.Start(context.Background(), "demo", cfg); err == nil {
		t.Fatal("Start() error = nil")
	}
	if containsEvent(events, "start demo") {
		t.Fatalf("session started after hook failure: %#v", events)
	}
}

func TestStopDeletesMultipleProjectsThenRunsTheirHooks(t *testing.T) {
	events := []string{}
	sessions := []string{}
	client := &fakeHerdr{events: &events, exists: true}
	manager := New(client, fakeShell{events: &events, sessions: &sessions})
	first := validConfig()
	first.BeforeStop = []string{"first prepare"}
	first.AfterStop = []string{"first cleanup"}
	second := validConfig()
	second.Root = "/second"
	second.BeforeStop = []string{"second prepare"}
	second.AfterStop = []string{"second cleanup"}

	if err := manager.Stop(context.Background(), "first", first); err != nil {
		t.Fatalf("Stop(first) error = %v", err)
	}
	if err := manager.Stop(context.Background(), "second", second); err != nil {
		t.Fatalf("Stop(second) error = %v", err)
	}

	want := []string{
		"exists first", "shell /project first prepare", "delete first", "shell /project first cleanup",
		"exists second", "shell /second second prepare", "delete second", "shell /second second cleanup",
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
	if want := []string{"first", "first", "second", "second"}; !reflect.DeepEqual(sessions, want) {
		t.Fatalf("hook sessions = %#v, want %#v", sessions, want)
	}
}

func TestDefaultShellOverridesAmbientHerdrSession(t *testing.T) {
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("HERDR_SESSION", "ambient")

	shell := NewDefaultShell(nil)
	if err := shell.Run(context.Background(), "demo", t.TempDir(), `[ "$HERDR_SESSION" = "demo" ]`); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestStopAbsentSessionIsNoOp(t *testing.T) {
	events := []string{}
	manager := New(&fakeHerdr{events: &events}, fakeShell{events: &events})
	cfg := validConfig()
	cfg.BeforeStop = []string{"must not run"}
	cfg.AfterStop = []string{"must not run"}

	if err := manager.Stop(context.Background(), "missing", cfg); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if want := []string{"exists missing"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
}

func TestStopPreservesSessionAfterBeforeStopFailure(t *testing.T) {
	events := []string{}
	client := &fakeHerdr{events: &events, exists: true}
	manager := New(client, fakeShell{events: &events, failAt: "shell /project prepare"})
	cfg := validConfig()
	cfg.BeforeStop = []string{"prepare"}

	err := manager.Stop(context.Background(), "demo", cfg)
	if err == nil || !strings.Contains(err.Error(), "before_stop") {
		t.Fatalf("Stop() error = %v", err)
	}
	if containsEvent(events, "delete demo") {
		t.Fatalf("session deleted after before_stop failure: %#v", events)
	}
}

func TestStopDoesNotRunHookAfterDeleteFailure(t *testing.T) {
	events := []string{}
	client := &fakeHerdr{events: &events, exists: true, failAt: "delete demo"}
	manager := New(client, fakeShell{events: &events})
	cfg := validConfig()
	cfg.AfterStop = []string{"must not run"}

	if err := manager.Stop(context.Background(), "demo", cfg); err == nil {
		t.Fatal("Stop() error = nil")
	}
	if containsEvent(events, "must not run") {
		t.Fatalf("after_stop ran after delete failure: %#v", events)
	}
}

func TestStopReportsAfterStopFailureAfterDeletion(t *testing.T) {
	events := []string{}
	client := &fakeHerdr{events: &events, exists: true}
	manager := New(client, fakeShell{events: &events, failAt: "shell /project cleanup"})
	cfg := validConfig()
	cfg.AfterStop = []string{"cleanup"}

	err := manager.Stop(context.Background(), "demo", cfg)
	if err == nil || !strings.Contains(err.Error(), "after_stop") {
		t.Fatalf("Stop() error = %v", err)
	}
	if !containsEvent(events, "delete demo") {
		t.Fatalf("session not deleted before failed hook: %#v", events)
	}
}

func validConfig() *config.Config {
	return &config.Config{
		Root: "/project",
		Workspaces: []config.Workspace{
			{Label: "api", Root: "/project/api", Tabs: []config.Tab{{Label: "shell", Root: "/project/api"}}},
		},
	}
}

type fakeHerdr struct {
	events     *[]string
	exists     bool
	failAt     string
	splitCount int
}

func (f *fakeHerdr) record(event string) error {
	*f.events = append(*f.events, event)
	if event == f.failAt {
		return errors.New("injected failure")
	}
	return nil
}

func (f *fakeHerdr) SessionExists(_ context.Context, name string) (bool, error) {
	return f.exists, f.record("exists " + name)
}

func (f *fakeHerdr) StartSession(_ context.Context, name string) error {
	return f.record("start " + name)
}

func (f *fakeHerdr) DeleteSession(_ context.Context, name string) error {
	return f.record("delete " + name)
}

func (f *fakeHerdr) CreateWorkspace(_ context.Context, session, label, root string) (herdr.WorkspaceResult, error) {
	err := f.record(fmt.Sprintf("workspace %s %s %s", session, label, root))
	return herdr.WorkspaceResult{WorkspaceID: "workspace-1", TabID: "tab-1", RootPaneID: "pane-1"}, err
}

func (f *fakeHerdr) RenameTab(_ context.Context, session, tabID, label string) error {
	return f.record(fmt.Sprintf("rename-tab %s %s %s", session, tabID, label))
}

func (f *fakeHerdr) CreateTab(_ context.Context, session, workspaceID, label, root string) (herdr.TabResult, error) {
	err := f.record(fmt.Sprintf("tab %s %s %s %s", session, workspaceID, label, root))
	return herdr.TabResult{TabID: "tab-2", RootPaneID: "pane-tab-2"}, err
}

func (f *fakeHerdr) SplitPane(_ context.Context, session, paneID, direction, root string, ratio *float64) (string, error) {
	f.splitCount++
	ratioText := "none"
	if ratio != nil {
		ratioText = fmt.Sprintf("%g", *ratio)
	}
	event := fmt.Sprintf("split %s %s %s %s %s", session, paneID, direction, root, ratioText)
	return fmt.Sprintf("pane-split-%d", f.splitCount), f.record(event)
}

func (f *fakeHerdr) RunPane(_ context.Context, session, paneID, command string) error {
	return f.record(fmt.Sprintf("run %s %s %s", session, paneID, command))
}

func (f *fakeHerdr) FocusWorkspace(_ context.Context, session, workspaceID string) error {
	return f.record(fmt.Sprintf("focus %s %s", session, workspaceID))
}

type fakeShell struct {
	events   *[]string
	sessions *[]string
	failAt   string
}

func (f fakeShell) Run(_ context.Context, session, root, command string) error {
	if f.sessions != nil {
		*f.sessions = append(*f.sessions, session)
	}
	event := fmt.Sprintf("shell %s %s", root, command)
	*f.events = append(*f.events, event)
	if event == f.failAt {
		return errors.New("injected failure")
	}
	return nil
}

func containsEvent(events []string, fragment string) bool {
	for _, event := range events {
		if strings.Contains(event, fragment) {
			return true
		}
	}
	return false
}
