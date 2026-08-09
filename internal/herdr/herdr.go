package herdr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tombell/tendr/internal/sshutil"
)

const sessionEnvironment = "HERDR_SESSION"

type Session struct {
	Name    string `json:"name"`
	Running bool   `json:"running"`
}

type WorkspaceResult struct {
	WorkspaceID string
	TabID       string
	RootPaneID  string
}

type TabResult struct {
	TabID      string
	RootPaneID string
}

type Client struct {
	binary        string
	remote        string
	logger        *log.Logger
	readyTimeout  time.Duration
	pollInterval  time.Duration
	commandRunner func(context.Context, string, []string) ([]byte, error)
}

func New(binary string, logger *log.Logger) Client {
	return newClient(binary, "", logger)
}

func NewRemote(binary, remote string, logger *log.Logger) Client {
	return newClient(binary, remote, logger)
}

func newClient(binary, remote string, logger *log.Logger) Client {
	if binary == "" {
		binary = "herdr"
	}
	client := Client{
		remote:       remote,
		binary:       binary,
		logger:       logger,
		readyTimeout: 5 * time.Second,
		pollInterval: 50 * time.Millisecond,
	}
	client.commandRunner = client.exec
	return client
}

func (c Client) ListSessions(ctx context.Context) ([]Session, error) {
	output, err := c.run(ctx, "", "session", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var response struct {
		Sessions []Session `json:"sessions"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("parse session list: %w", err)
	}
	return response.Sessions, nil
}

func (c Client) SessionExists(ctx context.Context, name string) (bool, error) {
	sessions, err := c.ListSessions(ctx)
	if err != nil {
		return false, err
	}
	for _, session := range sessions {
		if session.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (c Client) AttachSession(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) error {
	sessions, err := c.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("check session %q before attaching: %w", name, err)
	}

	found := false
	for _, session := range sessions {
		if session.Name == name {
			found = true
			if session.Running {
				break
			}
			return fmt.Errorf("session %q is not running", name)
		}
	}
	if !found {
		return fmt.Errorf("session %q does not exist", name)
	}

	if c.logger != nil {
		c.logger.Printf("%s session attach %s", c.binary, formatArguments([]string{name}))
	}

	args := []string{"session", "attach", name}
	if c.remote != "" {
		args = []string{"--remote", c.remote, "--session", name}
	}
	command := exec.CommandContext(ctx, c.binary, args...)
	command.Env = withSession(os.Environ(), "")
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("attach session %q: %w", name, err)
	}
	return nil
}

func (c Client) StartSession(ctx context.Context, name string) error {
	if c.remote != "" {
		if c.logger != nil {
			c.logger.Printf("ssh %s nohup env HERDR_SESSION=%s herdr server", c.remote, name)
		}
		remoteCommand := "nohup env " + shellQuote(sessionEnvironment+"="+name) + " herdr server >/dev/null 2>&1 </dev/null &"
		command, err := sshutil.Command(ctx, c.remote, remoteCommand)
		if err != nil {
			return err
		}
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("start remote server for session %q: %w: %s", name, err, strings.TrimSpace(string(output)))
		}
		return c.waitForServer(ctx, name, nil)
	}
	if c.logger != nil {
		c.logger.Printf("HERDR_SESSION=%s %s server", name, c.binary)
	}

	command := exec.Command(c.binary, "server")
	command.Env = withSession(os.Environ(), name)
	detachProcess(command)

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open null output for session %q: %w", name, err)
	}
	defer devNull.Close()
	command.Stdout = devNull
	command.Stderr = devNull
	if err := command.Start(); err != nil {
		return fmt.Errorf("start server for session %q: %w", name, err)
	}

	return c.waitForServer(ctx, name, command)
}

func (c Client) waitForServer(ctx context.Context, name string, command *exec.Cmd) error {
	deadline := time.NewTimer(c.readyTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		running, err := c.serverRunning(ctx, name)
		if err != nil {
			lastErr = err
		} else if running {
			if command != nil {
				if err := command.Process.Release(); err != nil {
					return fmt.Errorf("release server process for session %q: %w", name, err)
				}
			}
			return nil
		} else {
			lastErr = fmt.Errorf("server is not running")
		}

		select {
		case <-ctx.Done():
			if command != nil {
				stopProcess(command)
			}
			return fmt.Errorf("wait for session %q readiness: %w", name, ctx.Err())
		case <-deadline.C:
			if command != nil {
				stopProcess(command)
			}
			return fmt.Errorf("wait for session %q readiness: %w", name, lastErr)
		case <-ticker.C:
		}
	}
}

func (c Client) serverRunning(ctx context.Context, session string) (bool, error) {
	output, err := c.commandRunner(ctx, session, []string{"status", "server", "--json"})
	if err != nil {
		return false, err
	}

	var status struct {
		Running bool `json:"running"`
	}
	if err := json.Unmarshal(output, &status); err != nil {
		return false, fmt.Errorf("parse server status: %w", err)
	}
	return status.Running, nil
}

func (c Client) CreateWorkspace(ctx context.Context, session, label, root string) (WorkspaceResult, error) {
	output, err := c.run(ctx, session, "workspace", "create", "--cwd", root, "--label", label, "--no-focus")
	if err != nil {
		return WorkspaceResult{}, fmt.Errorf("create workspace %q: %w", label, err)
	}

	var response struct {
		Result struct {
			Workspace struct {
				ID string `json:"workspace_id"`
			} `json:"workspace"`
			Tab struct {
				ID string `json:"tab_id"`
			} `json:"tab"`
			RootPane struct {
				ID string `json:"pane_id"`
			} `json:"root_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return WorkspaceResult{}, fmt.Errorf("parse workspace %q response: %w", label, err)
	}
	result := WorkspaceResult{
		WorkspaceID: response.Result.Workspace.ID,
		TabID:       response.Result.Tab.ID,
		RootPaneID:  response.Result.RootPane.ID,
	}
	if result.WorkspaceID == "" || result.TabID == "" || result.RootPaneID == "" {
		return WorkspaceResult{}, fmt.Errorf("parse workspace %q response: missing returned workspace, tab, or pane ID", label)
	}
	return result, nil
}

func (c Client) RenameTab(ctx context.Context, session, tabID, label string) error {
	if _, err := c.run(ctx, session, "tab", "rename", tabID, label); err != nil {
		return fmt.Errorf("rename tab %q to %q: %w", tabID, label, err)
	}
	return nil
}

func (c Client) CreateTab(ctx context.Context, session, workspaceID, label, root string) (TabResult, error) {
	output, err := c.run(ctx, session, "tab", "create", "--workspace", workspaceID, "--cwd", root, "--label", label, "--no-focus")
	if err != nil {
		return TabResult{}, fmt.Errorf("create tab %q: %w", label, err)
	}

	var response struct {
		Result struct {
			Tab struct {
				ID string `json:"tab_id"`
			} `json:"tab"`
			RootPane struct {
				ID string `json:"pane_id"`
			} `json:"root_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return TabResult{}, fmt.Errorf("parse tab %q response: %w", label, err)
	}
	result := TabResult{TabID: response.Result.Tab.ID, RootPaneID: response.Result.RootPane.ID}
	if result.TabID == "" || result.RootPaneID == "" {
		return TabResult{}, fmt.Errorf("parse tab %q response: missing returned tab or pane ID", label)
	}
	return result, nil
}

func (c Client) SplitPane(ctx context.Context, session, paneID, direction, root string, ratio *float64) (string, error) {
	args := []string{"pane", "split", paneID, "--direction", direction, "--cwd", root, "--no-focus"}
	if ratio != nil {
		args = append(args, "--ratio", strconv.FormatFloat(*ratio, 'f', -1, 64))
	}
	output, err := c.run(ctx, session, args...)
	if err != nil {
		return "", fmt.Errorf("split pane %q %s: %w", paneID, direction, err)
	}

	var response struct {
		Result struct {
			Pane struct {
				ID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return "", fmt.Errorf("parse split pane %q response: %w", paneID, err)
	}
	if response.Result.Pane.ID == "" {
		return "", fmt.Errorf("parse split pane %q response: missing returned pane ID", paneID)
	}
	return response.Result.Pane.ID, nil
}

func (c Client) RunPane(ctx context.Context, session, paneID, command string) error {
	if _, err := c.run(ctx, session, "pane", "run", paneID, command); err != nil {
		return fmt.Errorf("run command in pane %q: %w", paneID, err)
	}
	return nil
}

func (c Client) FocusWorkspace(ctx context.Context, session, workspaceID string) error {
	if _, err := c.run(ctx, session, "workspace", "focus", workspaceID); err != nil {
		return fmt.Errorf("focus workspace %q: %w", workspaceID, err)
	}
	return nil
}

func (c Client) DeleteSession(ctx context.Context, name string) error {
	sessions, err := c.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("inspect session %q before deletion: %w", name, err)
	}
	var found bool
	var running bool
	for _, session := range sessions {
		if session.Name == name {
			found = true
			running = session.Running
			break
		}
	}
	if !found {
		return nil
	}
	if running {
		if _, err := c.run(ctx, "", "session", "stop", name, "--json"); err != nil {
			return fmt.Errorf("stop session %q before deletion: %w", name, err)
		}
	}
	if _, err := c.run(ctx, "", "session", "delete", name, "--json"); err != nil {
		return fmt.Errorf("delete session %q: %w", name, err)
	}
	return nil
}

func (c Client) run(ctx context.Context, session string, args ...string) ([]byte, error) {
	if c.logger != nil {
		prefix := ""
		if session != "" {
			prefix = sessionEnvironment + "=" + session + " "
		}
		c.logger.Printf("%s%s %s", prefix, c.binary, formatArguments(args))
	}
	return c.commandRunner(ctx, session, args)
}

func (c Client) exec(ctx context.Context, session string, args []string) ([]byte, error) {
	var command *exec.Cmd
	if c.remote == "" {
		command = exec.CommandContext(ctx, c.binary, args...)
		command.Env = withSession(os.Environ(), session)
	} else {
		remoteArgs := make([]string, 0, len(args)+2)
		if session != "" {
			remoteArgs = append(remoteArgs, "env", sessionEnvironment+"="+session)
		}
		remoteArgs = append(remoteArgs, "herdr")
		remoteArgs = append(remoteArgs, args...)
		var err error
		command, err = sshutil.Command(ctx, c.remote, joinShellArguments(remoteArgs))
		if err != nil {
			return nil, err
		}
	}
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	return output, nil
}

func withSession(environment []string, session string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, variable := range environment {
		if !strings.HasPrefix(variable, sessionEnvironment+"=") {
			result = append(result, variable)
		}
	}
	if session != "" {
		result = append(result, sessionEnvironment+"="+session)
	}
	return result
}

func stopProcess(command *exec.Cmd) {
	if command.Process == nil {
		return
	}
	_ = command.Process.Kill()
	_ = command.Wait()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func joinShellArguments(arguments []string) string {
	quoted := make([]string, len(arguments))
	for i, argument := range arguments {
		quoted[i] = shellQuote(argument)
	}
	return strings.Join(quoted, " ")
}

func formatArguments(arguments []string) string {
	formatted := make([]string, len(arguments))
	for index, argument := range arguments {
		if strings.ContainsAny(argument, " \t\n\"'") {
			formatted[index] = strconv.Quote(argument)
		} else {
			formatted[index] = argument
		}
	}
	return strings.Join(formatted, " ")
}
