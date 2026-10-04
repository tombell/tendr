package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tombell/tendr/internal/herdr"
	"github.com/tombell/tendr/internal/output"
)

type Client struct {
	target         string
	logger         *log.Logger
	nonInteractive bool
}

func New(target string, logger *log.Logger) (Client, error) {
	if target == "" || strings.HasPrefix(target, "-") || strings.ContainsFunc(target, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("-._@:/[]%+", r))
	}) {
		return Client{}, fmt.Errorf("invalid SSH target %q", target)
	}
	return Client{target: target, logger: logger}, nil
}

func (c Client) ForCompletion() Client {
	c.nonInteractive = true
	return c
}

func (c Client) Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if c.logger != nil {
		args = append([]string{"--debug"}, args...)
	}
	return c.exec(ctx, "tendr", args, stdout, stderr)
}

func (c Client) ListSessions(ctx context.Context) ([]herdr.Session, error) {
	var stdout bytes.Buffer
	var stderr output.Tail
	if err := c.exec(ctx, "herdr", []string{"session", "list", "--json"}, &stdout, &stderr); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	var response struct {
		Sessions []herdr.Session `json:"sessions"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("parse sessions on %q: %w", c.target, err)
	}
	return response.Sessions, nil
}

func (c Client) AttachSession(ctx context.Context, name string, stdin io.Reader, stdout, stderr io.Writer) error {
	sessions, err := c.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("check remote session %q before attaching: %w", name, err)
	}
	var found, running bool
	for _, session := range sessions {
		if session.Name == name {
			found, running = true, session.Running
			break
		}
	}
	if !found {
		return fmt.Errorf("session %q does not exist on %q", name, c.target)
	}
	if !running {
		return fmt.Errorf("session %q is not running on %q", name, c.target)
	}

	if c.logger != nil {
		c.logger.Printf("herdr --remote %q --session %q", c.target, name)
	}
	command := exec.CommandContext(ctx, "herdr", "--remote", c.target, "--session", name)
	command.Env = herdr.EnvironmentForSession(os.Environ(), "")
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("attach session %q on %q: %w", name, c.target, err)
	}
	return nil
}

func (c Client) exec(ctx context.Context, binary string, args []string, stdout, stderr io.Writer) error {
	// SSH passes its command through the remote user's shell. Quote each argument
	// there as well as keeping the local SSH arguments separate.
	words := []string{"env", "-u", "HERDR_SESSION", "-u", "HERDR_SOCKET_PATH", binary}
	words = append(words, args...)
	for index, word := range words {
		words[index] = "'" + strings.ReplaceAll(word, "'", "'\"'\"'") + "'"
	}
	remoteCommand := strings.Join(words, " ")
	sshArgs := []string{"-T"}
	if c.nonInteractive {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		sshArgs = append(sshArgs, "-o", "BatchMode=yes", "-o", "ConnectTimeout=5")
	}
	sshArgs = append(sshArgs, "--", c.target, remoteCommand)
	if c.logger != nil {
		c.logger.Printf("ssh %q", sshArgs)
	}
	command := exec.CommandContext(ctx, "ssh", sshArgs...)
	command.Env = herdr.EnvironmentForSession(os.Environ(), "")
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run %s on %q: %w", binary, c.target, err)
	}
	return nil
}
