package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tombell/tendr/internal/config"
	"github.com/tombell/tendr/internal/herdr"
	"github.com/tombell/tendr/internal/manager"
	"github.com/tombell/tendr/internal/remote"
)

type App struct {
	logger *log.Logger
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	remote *remote.Client
}

func New(logger *log.Logger, stdin io.Reader, stdout, stderr io.Writer) App {
	return App{logger: logger, stdin: stdin, stdout: stdout, stderr: stderr}
}

func (a App) WithTarget(target, machine string) (App, error) {
	if target != "" && machine != "" {
		return App{}, errors.New("--remote and --machine cannot be used together")
	}
	if machine != "" {
		profile, err := herdr.New("", a.logger).ResolveMachine(context.Background(), machine)
		if err != nil {
			return App{}, err
		}
		target = profile.Target
	}
	if target == "" {
		a.remote = nil
		return a, nil
	}
	client, err := remote.New(target, a.logger)
	if err != nil {
		return App{}, err
	}
	a.remote = &client
	return a, nil
}

func (a App) Complete(kind string) error {
	if a.remote != nil {
		client := a.remote.ForCompletion()
		a.remote = &client
	}
	switch kind {
	case "projects":
		return a.List()
	case "sessions":
		return a.ListRunningSessions()
	case "machines":
		return a.completeMachines()
	default:
		return errors.New("usage: tendr __complete <projects|sessions|machines>")
	}
}

func (a App) completeMachines() error {
	machines, err := herdr.New("", a.logger).ListMachines(context.Background())
	if err != nil {
		return err
	}
	labels := make(map[string]int)
	ids := make(map[string]struct{})
	for _, machine := range machines {
		labels[machine.Label]++
		ids[machine.ID] = struct{}{}
	}
	selectors := make(map[string]struct{})
	for _, machine := range machines {
		if !machine.Enabled {
			continue
		}
		selectors[machine.ID] = struct{}{}
		if _, isID := ids[machine.Label]; !isID && labels[machine.Label] == 1 {
			selectors[machine.Label] = struct{}{}
		}
	}
	var names []string
	for selector := range selectors {
		if selector != "" {
			names = append(names, selector)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintln(a.stdout, name); err != nil {
			return err
		}
	}
	return nil
}

func (a App) List() error {
	if a.remote != nil {
		return a.remote.Run(context.Background(), []string{"list"}, a.stdout, a.stderr)
	}
	dir, err := projectsDirectory()
	if err != nil {
		return fmt.Errorf("resolve projects directory: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read projects directory: %w", err)
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		projects = append(projects, strings.TrimSuffix(entry.Name(), ".yml"))
	}
	sort.Strings(projects)
	for _, project := range projects {
		fmt.Fprintln(a.stdout, project)
	}
	return nil
}

func (a App) ListRunningSessions() error {
	var sessions []herdr.Session
	var err error
	if a.remote != nil {
		sessions, err = a.remote.ListSessions(context.Background())
	} else {
		sessions, err = herdr.New("", a.logger).ListSessions(context.Background())
	}
	if err != nil {
		return fmt.Errorf("list running sessions: %w", err)
	}

	var names []string
	for _, session := range sessions {
		if session.Running {
			names = append(names, session.Name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintln(a.stdout, name); err != nil {
			return err
		}
	}
	return nil
}

func (a App) Start(projects []string, attach bool) error {
	if len(projects) == 0 {
		return errors.New("usage: tendr start [--attach] <project names...>")
	}
	if attach && len(projects) != 1 {
		return errors.New("--attach requires exactly one project")
	}
	if a.remote != nil {
		if err := validateRemoteProjects(projects); err != nil {
			return err
		}
		if err := a.remote.Run(context.Background(), append([]string{"start", "--"}, projects...), a.stdout, a.stderr); err != nil {
			return err
		}
		if attach {
			return a.Attach(projects[0])
		}
		return nil
	}

	loaded, err := loadProjects(projects)
	if err != nil {
		return err
	}
	client := herdr.New("", a.logger)
	lifecycle := manager.New(client, manager.NewDefaultShell(a.logger))
	for index, cfg := range loaded {
		if err := lifecycle.Start(context.Background(), projects[index], cfg); err != nil {
			return fmt.Errorf("start project %q: %w", projects[index], err)
		}
	}
	if attach {
		if err := client.AttachSession(context.Background(), projects[0], a.stdin, a.stdout, a.stderr); err != nil {
			return fmt.Errorf("attach project %q: %w", projects[0], err)
		}
	}
	return nil
}

func (a App) Attach(name string) error {
	if !validSessionName(name) {
		return fmt.Errorf("invalid session name %q", name)
	}
	if a.remote != nil {
		return a.remote.AttachSession(context.Background(), name, a.stdin, a.stdout, a.stderr)
	}

	client := herdr.New("", a.logger)
	return client.AttachSession(context.Background(), name, a.stdin, a.stdout, a.stderr)
}

func (a App) Stop(projects []string) error {
	if len(projects) == 0 {
		return errors.New("usage: tendr stop <project names...>")
	}
	if a.remote != nil {
		if err := validateRemoteProjects(projects); err != nil {
			return err
		}
		return a.remote.Run(context.Background(), append([]string{"stop", "--"}, projects...), a.stdout, a.stderr)
	}

	loaded, err := loadProjects(projects)
	if err != nil {
		return err
	}
	client := herdr.New("", a.logger)
	lifecycle := manager.New(client, manager.NewDefaultShell(a.logger))
	for index, cfg := range loaded {
		if err := lifecycle.Stop(context.Background(), projects[index], cfg); err != nil {
			return fmt.Errorf("stop project %q: %w", projects[index], err)
		}
	}
	return nil
}

func validateRemoteProjects(projects []string) error {
	var invalid []string
	for _, project := range projects {
		if !validProjectName(project) {
			invalid = append(invalid, fmt.Sprintf("%s (invalid project name)", project))
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("invalid projects: %s", strings.Join(invalid, ", "))
	}
	return nil
}

func loadProjects(projects []string) ([]*config.Config, error) {
	dir, err := projectsDirectory()
	if err != nil {
		return nil, fmt.Errorf("resolve projects directory: %w", err)
	}

	configs := make([]*config.Config, 0, len(projects))
	var invalid []string
	for _, project := range projects {
		if !validProjectName(project) {
			invalid = append(invalid, fmt.Sprintf("%s (invalid project name)", project))
			continue
		}
		cfg, err := config.Load(filepath.Join(dir, project+".yml"))
		if err != nil {
			invalid = append(invalid, fmt.Sprintf("%s (%v)", project, err))
			continue
		}
		configs = append(configs, cfg)
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("invalid projects: %s", strings.Join(invalid, ", "))
	}
	return configs, nil
}

func validProjectName(project string) bool {
	return project != "default" && validSessionName(project)
}

func validSessionName(name string) bool {
	if name == "" || len(name) > 64 || name == "." || name == ".." {
		return false
	}
	for _, character := range name {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '.', character == '_', character == '-':
		default:
			return false
		}
	}
	return true
}

func projectsDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tendr"), nil
}
