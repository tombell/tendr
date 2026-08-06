package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/tombell/tendr/internal/herdr"
)

const bashCompletion = `# bash completion for tendr
_tendr() {
  local current command candidate used
  local command_index i

  COMPREPLY=()
  current="${COMP_WORDS[COMP_CWORD]}"
  command=""
  command_index=-1

  for ((i = 1; i < COMP_CWORD; i++)); do
    case "${COMP_WORDS[i]}" in
      -d|--debug)
        ;;
      -v|--version|-h|--help)
        return
        ;;
      attach|completion|list|start|stop)
        command="${COMP_WORDS[i]}"
        command_index=$i
        break
        ;;
    esac
  done

  if [[ -z "$command" ]]; then
    COMPREPLY=( $(compgen -W 'attach completion list start stop -d --debug -v --version -h --help' -- "$current") )
    return
  fi

  case "$command" in
    attach)
      (( COMP_CWORD == command_index + 1 )) || return
      while IFS= read -r candidate; do
        if [[ -n "$candidate" && "$candidate" == "$current"* ]]; then
          COMPREPLY[${#COMPREPLY[@]}]="$candidate"
        fi
      done < <(tendr __complete sessions 2>/dev/null)
      ;;
    completion)
      if (( COMP_CWORD == command_index + 1 )); then
        COMPREPLY=( $(compgen -W 'bash zsh' -- "$current") )
      fi
      ;;
    start)
      candidates=(--attach)
      while IFS= read -r candidate; do
        [[ -n "$candidate" ]] && candidates+=("$candidate")
      done < <(tendr list 2>/dev/null)
      for candidate in "${candidates[@]}"; do
        [[ "$candidate" == "$current"* ]] || continue
        used=false
        for ((i = command_index + 1; i < COMP_CWORD; i++)); do
          if [[ "${COMP_WORDS[i]}" == "$candidate" ]]; then
            used=true
            break
          fi
        done
        if [[ "$used" == false ]]; then
          COMPREPLY[${#COMPREPLY[@]}]="$candidate"
        fi
      done
      ;;
    stop)
      while IFS= read -r candidate; do
        [[ -n "$candidate" && "$candidate" == "$current"* ]] || continue
        used=false
        for ((i = command_index + 1; i < COMP_CWORD; i++)); do
          if [[ "${COMP_WORDS[i]}" == "$candidate" ]]; then
            used=true
            break
          fi
        done
        if [[ "$used" == false ]]; then
          COMPREPLY[${#COMPREPLY[@]}]="$candidate"
        fi
      done < <(tendr list 2>/dev/null)
      ;;
  esac
}

complete -F _tendr tendr
`

const zshCompletion = `#compdef tendr

_tendr() {
  local command candidate
  local -i command_index i used
  local -a candidates projects sessions

  command=""
  command_index=-1

  for ((i = 2; i < CURRENT; i++)); do
    case "${words[i]}" in
      -d|--debug)
        ;;
      -v|--version|-h|--help)
        return
        ;;
      attach|completion|list|start|stop)
        command="${words[i]}"
        command_index=$i
        break
        ;;
    esac
  done

  if [[ -z "$command" ]]; then
    candidates=(attach completion list start stop -d --debug -v --version -h --help)
    compadd -- "${candidates[@]}"
    return
  fi

  case "$command" in
    attach)
      (( CURRENT == command_index + 1 )) || return
      sessions=("${(@f)$(tendr __complete sessions 2>/dev/null)}")
      compadd -- "${sessions[@]}"
      ;;
    completion)
      if (( CURRENT == command_index + 1 )); then
        compadd -- bash zsh
      fi
      ;;
    start)
      projects=(--attach "${(@f)$(tendr list 2>/dev/null)}")
      candidates=()
      for candidate in "${projects[@]}"; do
        [[ -n "$candidate" ]] || continue
        used=0
        for ((i = command_index + 1; i < CURRENT; i++)); do
          if [[ "${words[i]}" == "$candidate" ]]; then
            used=1
            break
          fi
        done
        (( used == 0 )) && candidates+=("$candidate")
      done
      compadd -- "${candidates[@]}"
      ;;
    stop)
      projects=("${(@f)$(tendr list 2>/dev/null)}")
      candidates=()
      for candidate in "${projects[@]}"; do
        [[ -n "$candidate" ]] || continue
        used=0
        for ((i = command_index + 1; i < CURRENT; i++)); do
          if [[ "${words[i]}" == "$candidate" ]]; then
            used=1
            break
          fi
        done
        (( used == 0 )) && candidates+=("$candidate")
      done
      compadd -- "${candidates[@]}"
      ;;
  esac
}

compdef _tendr tendr
`

func (a App) Completion(shell string) error {
	var script string
	switch shell {
	case "bash":
		script = bashCompletion
	case "zsh":
		script = zshCompletion
	default:
		return fmt.Errorf("unsupported shell %q (supported: bash, zsh)", shell)
	}

	_, err := fmt.Fprint(a.stdout, script)
	return err
}

func (a App) ListRunningSessions() error {
	sessions, err := herdr.New("", a.logger).ListSessions(context.Background())
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
