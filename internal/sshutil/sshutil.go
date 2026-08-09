package sshutil

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
)

// Command creates an OpenSSH command for either an SSH config host or an
// ssh://user@host:port URL.
func Command(ctx context.Context, target, remoteCommand string) (*exec.Cmd, error) {
	args := []string{}
	if parsed, err := url.Parse(target); err == nil && parsed.Scheme != "" {
		if parsed.Scheme != "ssh" || parsed.Hostname() == "" || parsed.Path != "" && parsed.Path != "/" {
			return nil, fmt.Errorf("invalid SSH target %q", target)
		}
		if parsed.Port() != "" {
			args = append(args, "-p", parsed.Port())
		}
		host := parsed.Hostname()
		if parsed.User != nil {
			host = parsed.User.Username() + "@" + host
		}
		args = append(args, host)
	} else {
		args = append(args, target)
	}
	args = append(args, remoteCommand)
	return exec.CommandContext(ctx, "ssh", args...), nil
}
