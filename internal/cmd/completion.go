package cmd

import "fmt"

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
        COMPREPLY=( $(compgen -W 'bash fish zsh' -- "$current") )
      fi
      ;;
    list)
      if (( COMP_CWORD == command_index + 1 )); then
        COMPREPLY=( $(compgen -W '--running' -- "$current") )
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
        compadd -- bash fish zsh
      fi
      ;;
    list)
      if (( CURRENT == command_index + 1 )); then
        compadd -- --running
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

const fishCompletion = `# fish completion for tendr
function __tendr_no_subcommand
    set -l tokens (commandline -xpc)
    for token in $tokens[2..-1]
        switch $token
            case -v --version -h --help attach completion list start stop
                return 1
        end
    end
    return 0
end

function __tendr_using_subcommand
    set -l tokens (commandline -xpc)
    for token in $tokens[2..-1]
        switch $token
            case -v --version -h --help
                return 1
            case attach completion list start stop
                test "$token" = "$argv[1]"
                return
        end
    end
    return 1
end

function __tendr_needs_argument
    __tendr_using_subcommand $argv[1]; or return 1
    set -l tokens (commandline -xpc)
    set -l command_index (contains -i -- $argv[1] $tokens)
    test -n "$command_index"; and test (count $tokens) -eq $command_index
end

function __tendr_projects
    set -l used (commandline -xpc)
    for project in (tendr list 2>/dev/null)
        if not contains -- $project $used
            string escape -- $project
        end
    end
end

function __tendr_running_sessions
    for session in (tendr __complete sessions 2>/dev/null)
        string escape -- $session
    end
end

complete -c tendr -f

complete -c tendr -n __tendr_no_subcommand -a attach -d 'Attach to a Herdr session'
complete -c tendr -n __tendr_no_subcommand -a completion -d 'Generate shell completion script'
complete -c tendr -n __tendr_no_subcommand -a list -d 'List configured projects or running sessions'
complete -c tendr -n __tendr_no_subcommand -a start -d 'Start Herdr project sessions'
complete -c tendr -n __tendr_no_subcommand -a stop -d 'Stop Herdr project sessions'
complete -c tendr -n __tendr_no_subcommand -s d -l debug -d 'Show debug logging'
complete -c tendr -n __tendr_no_subcommand -s v -l version -d 'Show the version number'
complete -c tendr -n __tendr_no_subcommand -s h -l help -d 'Show help'

complete -c tendr -n '__tendr_needs_argument attach' -a '(__tendr_running_sessions)'
complete -c tendr -n '__tendr_needs_argument completion' -a 'bash fish zsh'
complete -c tendr -n '__tendr_needs_argument list' -l running -d 'List running sessions'
complete -c tendr -n '__tendr_using_subcommand start' -l attach -d 'Attach after starting'
complete -c tendr -n '__tendr_using_subcommand start' -a '(__tendr_projects)'
complete -c tendr -n '__tendr_using_subcommand stop' -a '(__tendr_projects)'
`

func (a App) Completion(shell string) error {
	var script string
	switch shell {
	case "bash":
		script = bashCompletion
	case "fish":
		script = fishCompletion
	case "zsh":
		script = zshCompletion
	default:
		return fmt.Errorf("unsupported shell %q (supported: bash, fish, zsh)", shell)
	}

	_, err := fmt.Fprint(a.stdout, script)
	return err
}
