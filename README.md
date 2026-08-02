# Tendr

Tendr is an opinionated Go CLI for declaratively managing local [Herdr](https://herdr.dev/) projects. Each `~/.config/tendr/<project>.yml` file owns one named Herdr session. Starting a project creates its workspaces, tabs, pane topology, commands, and lifecycle hooks; stopping it deletes the named session and its persisted state.

Tendr follows the shape and fresh-restart workflow of [tm](https://github.com/tombell/tm), translated into Herdr's native concepts and vocabulary:

| Tendr | Herdr |
| --- | --- |
| project/config file | named session |
| workspace | workspace |
| tab | tab |
| pane | pane |

Tendr only manages local sessions in v1. It selects every session-scoped CLI call with `HERDR_SESSION` and uses IDs returned by Herdr's JSON responses rather than deriving IDs.

## Tendr and herdr-spreader

Tendr owns a named session's lifecycle: it starts a headless server, creates a project only when that named session does not already exist, runs project/workspace hooks, and deletes the whole named session on `tendr stop`. One command can validate and manage several project files.

[herdr-spreader](https://github.com/yuk1ty/herdr-spreader) is a Herdr plugin or standalone layout applicator for a server that is already running. It has richer layout-application features such as scoped environment variables, output waits, explicit pane focus, nested split chains, and dry runs. Use herdr-spreader when you want to apply layouts inside an existing session; use Tendr when the config file should own a separate named session and its start/stop lifecycle.

## Install

Herdr must be installed and available as `herdr` on `PATH`.

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
tendr start <project names...>
tendr stop <project names...>
tendr --debug start <project names...>
tendr --version
```

`start` loads and strictly validates every requested project before it changes Herdr state. Existing named sessions are skipped, making repeated starts idempotent. If creation fails after a session has started, Tendr deliberately leaves that partial session in place for diagnosis.

`stop` uses `herdr session delete`, not `herdr server stop`. It removes the named session and persisted state, then runs the project's `after_stop` hooks. A project whose session does not exist is a no-op.

## Configuration

Create `~/.config/tendr/<project>.yml`. The filename without `.yml` is the named Herdr session. Tendr rejects unknown fields and tm compatibility aliases.

```yaml
root: ~/Code/acme

before_start:
  - mise install

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

Paths resolve from parent to child: project `root` → workspace `root` → tab `root` → pane `root`. An omitted child root inherits its parent; an absolute path replaces it; `~` expands to the current user's home directory. The project root, at least one workspace, and at least one tab per workspace are required. Labels must be unique among sibling workspaces or tabs. Pane `direction` is `right` or `down`; an optional `ratio` must be greater than `0` and less than `1`.

Project hooks run in the project root. Workspace hooks run in the workspace root. Tab commands run in the tab's root pane; each configured pane is split from that root pane and receives its own commands through `herdr pane run`.

## Migrating from tm

Tendr does not read tm configs. Translate them explicitly:

| tm | Tendr | Notes |
| --- | --- | --- |
| config filename | config filename | Becomes the Herdr named session. |
| project `root` | project `root` | Same inherited base-path role. |
| project `before_start` | project `before_start` | Runs before the named server starts. |
| project `after_stop` | project `after_stop` | Runs after the named session is deleted. |
| `sessions[]` | `workspaces[]` | A tm session becomes a Herdr workspace. |
| session `name` | workspace `label` | Herdr-native label vocabulary. |
| session `root` | workspace `root` | Relative to project root. |
| session `before_start` / `after_start` | workspace `before_start` / `after_start` | Same lifecycle points. |
| `windows[]` | `tabs[]` | A tm window becomes a Herdr tab. |
| window `name` | tab `label` | Herdr-native label vocabulary. |
| window `commands` | tab `commands` | Runs in the returned root pane. |
| window `layout` | removed | Choose explicit pane directions and ratios instead. |
| pane `type` | pane `direction` | Choose Herdr `right` or `down`; there is no horizontal/vertical alias. |
| pane `root` / `commands` | pane `root` / `commands` | Uses the returned split-pane ID. |

## Development

```sh
gofmt -w .
go test ./... -count=1
go vet ./...
make prod
```

`make` injects `VERSION` and the current commit into the binary. `make prod` builds Darwin and Linux binaries for amd64 and arm64.
