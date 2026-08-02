package herdr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientUsesReturnedIDsAndTargetsNamedSession(t *testing.T) {
	client, logPath := newFakeClient(t)
	ctx := context.Background()

	workspace, err := client.CreateWorkspace(ctx, "demo", "api", "/code/api")
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	if workspace != (WorkspaceResult{WorkspaceID: "ws-returned", TabID: "tab-returned", RootPaneID: "pane-returned"}) {
		t.Fatalf("CreateWorkspace() = %#v", workspace)
	}
	if err := client.RenameTab(ctx, "demo", workspace.TabID, "server"); err != nil {
		t.Fatalf("RenameTab() error = %v", err)
	}
	tab, err := client.CreateTab(ctx, "demo", workspace.WorkspaceID, "logs", "/code/api")
	if err != nil {
		t.Fatalf("CreateTab() error = %v", err)
	}
	ratio := 0.4
	paneID, err := client.SplitPane(ctx, "demo", tab.RootPaneID, "right", "/code/api/tests", &ratio)
	if err != nil {
		t.Fatalf("SplitPane() error = %v", err)
	}
	if paneID != "split-returned" {
		t.Fatalf("SplitPane() = %q", paneID)
	}
	if err := client.RunPane(ctx, "demo", paneID, "go test ./..."); err != nil {
		t.Fatalf("RunPane() error = %v", err)
	}
	if err := client.FocusWorkspace(ctx, "demo", workspace.WorkspaceID); err != nil {
		t.Fatalf("FocusWorkspace() error = %v", err)
	}

	log := readLog(t, logPath)
	for _, want := range []string{
		"demo|workspace create --cwd /code/api --label api --no-focus",
		"demo|tab rename tab-returned server",
		"demo|tab create --workspace ws-returned --cwd /code/api --label logs --no-focus",
		"demo|pane split tab-root-returned --direction right --cwd /code/api/tests --no-focus --ratio 0.4",
		"demo|pane run split-returned go test ./...",
		"demo|workspace focus ws-returned",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("fake log missing %q:\n%s", want, log)
		}
	}
}

func TestSessionCommandsDoNotLeakAmbientSession(t *testing.T) {
	client, logPath := newFakeClient(t)
	t.Setenv(sessionEnvironment, "ambient")
	ctx := context.Background()

	sessions, err := client.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 2 || sessions[0].Name != "demo" || !sessions[0].Running {
		t.Fatalf("ListSessions() = %#v", sessions)
	}
	exists, err := client.SessionExists(ctx, "saved")
	if err != nil || !exists {
		t.Fatalf("SessionExists(saved) = %v, %v", exists, err)
	}
	if err := client.DeleteSession(ctx, "demo"); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	log := readLog(t, logPath)
	if strings.Contains(log, "ambient|") {
		t.Fatalf("ambient HERDR_SESSION leaked:\n%s", log)
	}
	if !strings.Contains(log, "unset|session delete demo --json") {
		t.Fatalf("delete was unexpectedly session-targeted:\n%s", log)
	}
}

func TestStartSessionPollsUntilReady(t *testing.T) {
	client, logPath := newFakeClient(t)
	client.readyTimeout = time.Second
	client.pollInterval = time.Millisecond

	if err := client.StartSession(context.Background(), "new-session"); err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	log := readLog(t, logPath)
	if !strings.Contains(log, "new-session|server") || strings.Count(log, "new-session|status server") < 2 {
		t.Fatalf("server was not started and polled:\n%s", log)
	}
}

func TestCreateWorkspaceRejectsMissingIDs(t *testing.T) {
	client, _ := newFakeClient(t)
	client.commandRunner = func(context.Context, string, []string) ([]byte, error) {
		return []byte(`{"result":{"workspace":{"workspace_id":"w1"}}}`), nil
	}

	_, err := client.CreateWorkspace(context.Background(), "demo", "api", "/tmp")
	if err == nil || !strings.Contains(err.Error(), "missing returned workspace, tab, or pane ID") {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
}

func TestListSessionsRejectsMalformedJSON(t *testing.T) {
	client, _ := newFakeClient(t)
	client.commandRunner = func(context.Context, string, []string) ([]byte, error) {
		return []byte("not json"), nil
	}

	if _, err := client.ListSessions(context.Background()); err == nil || !strings.Contains(err.Error(), "parse session list") {
		t.Fatalf("ListSessions() error = %v", err)
	}
}

func newFakeClient(t *testing.T) (Client, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "commands.log")
	statePath := filepath.Join(dir, "status-count")
	scriptPath := filepath.Join(dir, "herdr")
	script := `#!/bin/sh
session="${HERDR_SESSION-unset}"
printf '%s|%s\n' "$session" "$*" >> "$FAKE_HERDR_LOG"
case "$*" in
  "session list --json")
    printf '%s\n' '{"sessions":[{"name":"demo","running":true},{"name":"saved","running":false}]}'
    ;;
  "server")
    exit 0
    ;;
  "status server")
    count=0
    if [ -f "$FAKE_HERDR_STATE" ]; then count=$(sed -n '1p' "$FAKE_HERDR_STATE"); fi
    count=$((count + 1))
    printf '%s\n' "$count" > "$FAKE_HERDR_STATE"
    if [ "$count" -lt 2 ]; then exit 1; fi
    printf '%s\n' '{"running":true}'
    ;;
  "workspace create --cwd /code/api --label api --no-focus")
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"ws-returned"},"tab":{"tab_id":"tab-returned"},"root_pane":{"pane_id":"pane-returned"}}}'
    ;;
  "tab create --workspace ws-returned --cwd /code/api --label logs --no-focus")
    printf '%s\n' '{"result":{"tab":{"tab_id":"tab-created"},"root_pane":{"pane_id":"tab-root-returned"}}}'
    ;;
  "pane split tab-root-returned --direction right --cwd /code/api/tests --no-focus --ratio 0.4")
    printf '%s\n' '{"result":{"pane":{"pane_id":"split-returned"}}}'
    ;;
  *)
    printf '%s\n' '{"result":{"type":"ok"}}'
    ;;
esac
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(fake herdr) error = %v", err)
	}
	t.Setenv("FAKE_HERDR_LOG", logPath)
	t.Setenv("FAKE_HERDR_STATE", statePath)
	return New(scriptPath, nil), logPath
}

func readLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}
