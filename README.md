# Tendr

Tendr is a Go CLI for declaratively managing local [Herdr](https://herdr.dev/) projects. Each `~/.config/tendr/<name>.yml` file defines one named Herdr session, including its workspaces, tabs, panes, commands, and lifecycle hooks.

## Install

Install Herdr and make sure `herdr` is available on `PATH`, then install Tendr:

```sh
go install github.com/tombell/tendr/cmd/tendr@latest
```

For a local checkout:

```sh
make dev
```

## Usage

```text
tendr list
tendr start <names...>
tendr attach <name>
tendr stop <names...>
tendr --debug start <names...>
tendr --version
```

- `list` prints the configured project names.
- `start` validates every requested config, then creates any sessions that do not already exist.
- `attach` connects the current terminal to an existing session.
- `stop` deletes each session and its persisted state, then runs its `after_stop` hooks.

## Configuration

Create `~/.config/tendr/<name>.yml`. The filename without `.yml` becomes the Herdr session name.

```yaml
root: ~/Code/acme

before_start:
  - mise install

after_start:
  - echo "acme ready"

after_stop:
  - echo "acme stopped"

workspaces:
  - label: app
    root: .
    before_start:
      - make generate
    after_start:
      - echo "app ready"
    tabs:
      - label: server
        root: ./cmd/server
        commands:
          - go run .
        panes:
          - direction: right
            ratio: 0.4
            root: ../../
            commands:
              - go test ./... -count=1
          - direction: down
            root: ../../
            commands:
              - tail -f var/app.log
      - label: editor
        commands:
          - nvim .
```

See [`examples/project.yml`](examples/project.yml) for a standalone example.

Roots inherit from project → workspace → tab → pane. Relative paths resolve from the parent root, absolute paths replace it, and `~` expands to the current user's home directory.

Each project requires a root and at least one workspace. Each workspace requires at least one tab. Workspace and tab labels must be unique among siblings. Pane directions are `right` or `down`; optional ratios must be greater than `0` and less than `1`.

Project hooks run in the project root, workspace hooks in the workspace root, and commands in their tab or pane root. Project `after_start` hooks run once after all workspaces have started successfully.

## Development

```sh
gofmt -w .
go test ./... -count=1
go vet ./...
make prod
```

`make prod` builds Darwin and Linux binaries for amd64 and arm64.
