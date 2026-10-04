package remote

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewValidatesSSHTargets(t *testing.T) {
	for _, target := range []string{"workbox", "you@server", "ssh://you@server:2222", "you@[::1]", "ssh://you@[::1]:2222"} {
		if _, err := New(target, nil); err != nil {
			t.Errorf("New(%q) error = %v", target, err)
		}
	}
	for _, target := range []string{"", "-Fconfig", "host name", "host\nname", "host\x00name", "host;command", "host$(command)", "host`command`"} {
		if _, err := New(target, nil); err == nil {
			t.Errorf("New(%q) accepted invalid target", target)
		}
	}
}

func TestRunPreservesArgumentsThroughRemoteShell(t *testing.T) {
	dir := t.TempDir()
	ssh := `#!/bin/sh
exec /bin/sh -c "$4"
`
	tendr := `#!/bin/sh
test -z "${HERDR_SESSION-}" && test -z "${HERDR_SOCKET_PATH-}" || exit 9
printf '%s\n' "$@"
`
	for name, script := range map[string]string{"ssh": ssh, "tendr": tendr} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HERDR_SESSION", "ambient")
	t.Setenv("HERDR_SOCKET_PATH", "/ambient.sock")
	client, err := New("workbox", nil)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"start", "spaces and 'quotes'", "$(exit 1); `exit 2`", "--option=value"}
	var stdout, stderr bytes.Buffer
	if err := client.Run(context.Background(), args, &stdout, &stderr); err != nil {
		t.Fatalf("Run() error = %v, stderr = %q", err, &stderr)
	}
	if got, want := stdout.String(), strings.Join(args, "\n")+"\n"; got != want {
		t.Fatalf("remote arguments = %q, want %q", got, want)
	}
}

func TestListSessionsReportsSSHFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\nprintf 'Permission denied\\n' >&2\nexit 255\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	client, err := New("workbox", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListSessions(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Permission denied") || !strings.Contains(err.Error(), "workbox") {
		t.Fatalf("ListSessions() error = %v", err)
	}
}
