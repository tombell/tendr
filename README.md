# Tendr

Tendr is a Go CLI for declaratively managing local and remote [Herdr](https://herdr.dev/) projects. Each `~/.config/tendr/<name>.yml` file defines one named Herdr session, including its workspaces, tabs, panes, commands, and lifecycle hooks.

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
tendr list [--running]
tendr start <names...>
tendr start --attach <name>
tendr attach <name>
tendr stop <names...>
tendr completion <bash|fish|zsh>
tendr --debug start <names...>
tendr --version
```

- `list` prints the configured project names. Pass `--running` to print currently running Herdr sessions instead.
- `start` validates every requested config, then creates any sessions that do not already exist. Pass `--attach` with one project to connect the current terminal after startup finishes.
- `attach` connects the current terminal to an existing session.
- `stop` runs each session's `before_stop` hooks, deletes the session and its persisted state, then runs its `after_stop` hooks.

## Shell completion

Tendr can generate completion scripts for Bash, Fish and Zsh. The scripts complete commands and flags (including `start --attach`), configured projects for `start` and `stop`, and currently running Herdr sessions for `attach`.

For Bash, add this to `~/.bashrc`:

```sh
source <(tendr completion bash)
```

For Zsh, initialize its completion system and source the generated script from `~/.zshrc`:

```zsh
autoload -Uz compinit && compinit
source <(tendr completion zsh)
```

For Fish, source the generated script from `~/.config/fish/config.fish`:

```fish
tendr completion fish | source
```

## Configuration

Create `~/.config/tendr/<name>.yml`. The filename without `.yml` becomes the Herdr session name.

```yaml
root: ~/Code/acme

before_start:
  - mise install

after_start:
  - echo "acme ready"

before_stop:
  - echo "stopping acme"

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

### Remote projects

Set `remote` to an OpenSSH host alias or SSH URL to create and manage the project on another machine:

```yaml
remote: workbox
root: /home/you/Code/acme
workspaces:
  - label: app
    tabs:
      - label: shell
        commands:
          - make dev
```

Tendr executes Herdr operations and lifecycle hooks over `ssh`, while `tendr attach` and `tendr start --attach` use Herdr's local thin client (`herdr --remote workbox --session <name>`). Remote project roots must be absolute paths; nested relative roots still inherit normally. Herdr must be installed on the remote host and available to non-interactive SSH commands. Configure authentication in `~/.ssh/config`, load passphrase-protected keys with `ssh-add`, and verify `ssh workbox` before using Tendr. See [Herdr's persistence and remote access documentation](https://herdr.dev/docs/persistence-remote/).

`list --running` intentionally lists local sessions only; configured remote projects remain available through `list`.

Each project requires a root and at least one workspace. Each workspace requires at least one tab. Workspace and tab labels must be unique among siblings. Pane directions are `right` or `down`; optional ratios must be greater than `0` and less than `1`.

Project hooks run in the project root, workspace hooks in the workspace root, and commands in their tab or pane root. Tendr sets `HERDR_SESSION` to the project's session name for every lifecycle hook, overriding any inherited value. Project `after_start` hooks run once after all workspaces have started successfully. Project `before_stop` hooks must succeed before the session is deleted.

## Development

```sh
gofmt -w .
go test ./... -count=1
go vet ./...
make prod
```

`make prod` builds Darwin and Linux binaries for amd64 and arm64.
