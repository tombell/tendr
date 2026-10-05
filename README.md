# Tendr

Tendr is a Go CLI for declaratively managing [Herdr](https://herdr.dev/) projects locally or over SSH. Each `~/.config/tendr/<name>.yml` file defines one named Herdr session, including its workspaces, tabs, panes, commands, and lifecycle hooks.

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
tendr --remote <ssh-target> start --attach <name>
tendr --remote <ssh-target> attach <name>
tendr --machine <label-or-id> start --attach <name>
tendr --machine <label-or-id> attach <name>
tendr completion <bash|fish|zsh>
tendr --debug start <names...>
tendr --version
```

- `list` prints the configured project names. Pass `--running` to print currently running Herdr sessions instead.
- `start` validates every requested config, then creates any sessions that do not already exist. Pass `--attach` with one project to connect the current terminal after startup finishes.
- `attach` connects the current terminal to an existing session.
- `stop` runs each session's `before_stop` hooks, deletes the session and its persisted state, then runs its `after_stop` hooks.

## Remote sessions

Use `--remote` with an SSH config alias, `user@host`, or an SSH URI such as `ssh://you@server:2222`:

```sh
tendr --remote workbox list
tendr --remote workbox start --attach acme
tendr --remote workbox attach acme
tendr --remote workbox list --running
tendr --remote workbox stop acme
```

The flag also works after the command, before project names: `tendr start --remote workbox --attach acme`.

To select a saved [Herdr machine](https://herdr.dev/docs/connecting-machines/), use `--machine` with a profile ID or a unique, case-sensitive label from `herdr machine list`:

```sh
tendr --machine "Build machine" start --attach acme
tendr --machine "Build machine" attach acme
tendr --machine "Build machine" list --running
tendr stop --machine "Build machine" acme
```

Tendr reads the local catalog through `herdr machine list --json` and uses the profile's SSH target. The project name still chooses the Tendr session; the profile's saved remote session does not override it. Disabled, unknown, or ambiguous machines fail before connecting. Use either `--machine` or `--remote` in one command.

Install Tendr and Herdr on the remote Linux or macOS host and make both available on the `PATH` used by SSH commands. Put project YAML files in the remote user's `~/.config/tendr/` directory. Roots, pane commands, and lifecycle hooks all run on that host. Local YAML files are not read or copied when `--remote` is set.

Starting and stopping run remote Tendr through `ssh`. Starting multiple projects validates all their remote configs before creating sessions. With `--attach`, startup must succeed before the local Herdr client connects using `herdr --remote <ssh-target> --session <name>`. Attaching checks that the remote session exists and is running first; it does not need Tendr installed remotely. See [Herdr's remote access documentation](https://herdr.dev/docs/persistence-remote/) for client compatibility and keybindings.

SSH uses your existing authentication and host configuration. Verify access with `ssh workbox`; load passphrase-protected keys into `ssh-agent` when your terminal cannot show a passphrase prompt. Detaching the local client leaves the remote session running.

## Shell completion

Tendr can generate completion scripts for Bash, Fish and Zsh. The scripts complete commands and flags (including `start --attach`, `--remote`, and `--machine`), configured projects for `start`, and currently running Herdr sessions for `attach` and `stop`. Machine completion offers enabled profile IDs and unambiguous labels. When `--remote` or `--machine` is present, project and session completions query that SSH host without authentication prompts and with a five-second timeout.

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

Project names must contain between 1 and 64 ASCII characters using letters, digits, `.`, `_`, or `-`. The names `.`, `..`, and `default` are reserved. Use `tendr attach default` to attach to Herdr's default session.

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

Each project requires a root and at least one workspace. Each workspace requires at least one tab. Workspace and tab labels must be unique among siblings. Pane directions are `right` or `down`; optional ratios must be finite, greater than `0`, and less than `1`.

Project hooks run in the project root, workspace hooks in the workspace root, and commands in their tab or pane root. Tendr clears inherited `HERDR_SOCKET_PATH` overrides for Herdr commands and lifecycle hooks, and sets `HERDR_SESSION` to the project's session name for every lifecycle hook. Project `after_start` hooks run once after all workspaces have started successfully. Project `before_stop` hooks must succeed before the session is deleted.
