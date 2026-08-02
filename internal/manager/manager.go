package manager

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/tombell/tendr/internal/config"
	"github.com/tombell/tendr/internal/herdr"
)

type Herdr interface {
	SessionExists(context.Context, string) (bool, error)
	StartSession(context.Context, string) error
	DeleteSession(context.Context, string) error
	CreateWorkspace(context.Context, string, string, string) (herdr.WorkspaceResult, error)
	RenameTab(context.Context, string, string, string) error
	CreateTab(context.Context, string, string, string, string) (herdr.TabResult, error)
	SplitPane(context.Context, string, string, string, string, *float64) (string, error)
	RunPane(context.Context, string, string, string) error
	FocusWorkspace(context.Context, string, string) error
}

type Shell interface {
	Run(context.Context, string, string) error
}

type Manager struct {
	herdr Herdr
	shell Shell
}

func New(client Herdr, shell Shell) Manager {
	return Manager{herdr: client, shell: shell}
}

func (m Manager) Start(ctx context.Context, project string, cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	exists, err := m.herdr.SessionExists(ctx, project)
	if err != nil {
		return fmt.Errorf("check session %q: %w", project, err)
	}
	if exists {
		return nil
	}

	if err := m.runHooks(ctx, cfg.Root, cfg.BeforeStart); err != nil {
		return fmt.Errorf("run project before_start: %w", err)
	}
	if err := m.herdr.StartSession(ctx, project); err != nil {
		return err
	}

	for index := range cfg.Workspaces {
		if err := m.createWorkspace(ctx, project, &cfg.Workspaces[index]); err != nil {
			return fmt.Errorf("create workspace %q: %w", cfg.Workspaces[index].Label, err)
		}
	}
	return nil
}

func (m Manager) Stop(ctx context.Context, project string, cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	exists, err := m.herdr.SessionExists(ctx, project)
	if err != nil {
		return fmt.Errorf("check session %q: %w", project, err)
	}
	if !exists {
		return nil
	}
	if err := m.herdr.DeleteSession(ctx, project); err != nil {
		return err
	}
	if err := m.runHooks(ctx, cfg.Root, cfg.AfterStop); err != nil {
		return fmt.Errorf("run project after_stop: %w", err)
	}
	return nil
}

func (m Manager) createWorkspace(ctx context.Context, session string, workspace *config.Workspace) error {
	if err := m.runHooks(ctx, workspace.Root, workspace.BeforeStart); err != nil {
		return fmt.Errorf("run before_start: %w", err)
	}

	firstTab := &workspace.Tabs[0]
	created, err := m.herdr.CreateWorkspace(ctx, session, workspace.Label, firstTab.Root)
	if err != nil {
		return err
	}
	if err := m.herdr.RenameTab(ctx, session, created.TabID, firstTab.Label); err != nil {
		return err
	}
	if err := m.populateTab(ctx, session, created.RootPaneID, firstTab); err != nil {
		return fmt.Errorf("populate tab %q: %w", firstTab.Label, err)
	}

	for index := 1; index < len(workspace.Tabs); index++ {
		tab := &workspace.Tabs[index]
		createdTab, err := m.herdr.CreateTab(ctx, session, created.WorkspaceID, tab.Label, tab.Root)
		if err != nil {
			return err
		}
		if err := m.populateTab(ctx, session, createdTab.RootPaneID, tab); err != nil {
			return fmt.Errorf("populate tab %q: %w", tab.Label, err)
		}
	}

	if err := m.herdr.FocusWorkspace(ctx, session, created.WorkspaceID); err != nil {
		return err
	}
	if err := m.runHooks(ctx, workspace.Root, workspace.AfterStart); err != nil {
		return fmt.Errorf("run after_start: %w", err)
	}
	return nil
}

func (m Manager) populateTab(ctx context.Context, session, rootPaneID string, tab *config.Tab) error {
	if err := m.runPaneCommands(ctx, session, rootPaneID, tab.Commands); err != nil {
		return err
	}
	for index := range tab.Panes {
		pane := &tab.Panes[index]
		paneID, err := m.herdr.SplitPane(ctx, session, rootPaneID, pane.Direction, pane.Root, pane.Ratio)
		if err != nil {
			return err
		}
		if err := m.runPaneCommands(ctx, session, paneID, pane.Commands); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) runPaneCommands(ctx context.Context, session, paneID string, commands []string) error {
	for index, command := range commands {
		if err := m.herdr.RunPane(ctx, session, paneID, command); err != nil {
			return fmt.Errorf("command %d: %w", index+1, err)
		}
	}
	return nil
}

func (m Manager) runHooks(ctx context.Context, root string, hooks []string) error {
	for index, hook := range hooks {
		if err := m.shell.Run(ctx, root, hook); err != nil {
			return fmt.Errorf("hook %d: %w", index+1, err)
		}
	}
	return nil
}

type DefaultShell struct {
	logger *log.Logger
}

func NewDefaultShell(logger *log.Logger) DefaultShell {
	return DefaultShell{logger: logger}
}

func (s DefaultShell) Run(ctx context.Context, root, command string) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	if s.logger != nil {
		s.logger.Printf("cd %q && %s -c %q", root, shell, command)
	}

	process := exec.CommandContext(ctx, shell, "-c", command)
	process.Dir = root
	output, err := process.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}
