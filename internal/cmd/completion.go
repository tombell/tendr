package cmd

import "fmt"

const bashCompletion = `# bash completion for tendr
_tendr() {
  local current command candidate used remote machine
  local command_index i positional
  local -a candidates remote_args

  COMPREPLY=()
  current="${COMP_WORDS[COMP_CWORD]}"
  command=""
  command_index=-1
  remote=""
  machine=""
  positional=0
  if [[ "${COMP_WORDS[COMP_CWORD-1]}" == --machine ]]; then
    while IFS= read -r candidate; do
      [[ -n "$candidate" && "$candidate" == "$current"* ]] && COMPREPLY[${#COMPREPLY[@]}]="$candidate"
    done < <(tendr __complete machines 2>/dev/null)
    return
  fi
  [[ "${COMP_WORDS[COMP_CWORD-1]}" == --remote || "$current" == --remote=* || "$current" == --machine=* ]] && return

  for ((i = 1; i < COMP_CWORD; i++)); do
    case "${COMP_WORDS[i]}" in
      -d|--debug)
        ;;
      --remote)
        ((i++))
        remote="${COMP_WORDS[i]}"
        ;;
      --remote=*)
        remote="${COMP_WORDS[i]#--remote=}"
        ;;
      --machine)
        ((i++))
        machine="${COMP_WORDS[i]}"
        ;;
      --machine=*)
        machine="${COMP_WORDS[i]#--machine=}"
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
    COMPREPLY=( $(compgen -W 'attach completion list start stop -d --debug --remote --machine -v --version -h --help' -- "$current") )
    return
  fi

  for ((i = command_index + 1; i < COMP_CWORD; i++)); do
    case "${COMP_WORDS[i]}" in
      --remote) ((i++)); remote="${COMP_WORDS[i]}" ;;
      --remote=*) remote="${COMP_WORDS[i]#--remote=}" ;;
      --machine) ((i++)); machine="${COMP_WORDS[i]}" ;;
      --machine=*) machine="${COMP_WORDS[i]#--machine=}" ;;
      --attach|--running) ;;
      *) ((positional++)) ;;
    esac
  done
  remote_args=()
  [[ -n "$remote" ]] && remote_args=(--remote "$remote")
  [[ -n "$machine" ]] && remote_args+=(--machine "$machine")

  case "$command" in
    attach)
      (( positional == 0 )) || return
      if [[ "$current" == -* ]]; then
        COMPREPLY=( $(compgen -W '--remote --machine' -- "$current") )
        return
      fi
      while IFS= read -r candidate; do
        if [[ -n "$candidate" && "$candidate" == "$current"* ]]; then
          COMPREPLY[${#COMPREPLY[@]}]="$candidate"
        fi
      done < <(tendr "${remote_args[@]}" __complete sessions 2>/dev/null)
      ;;
    completion)
      if (( COMP_CWORD == command_index + 1 )); then
        COMPREPLY=( $(compgen -W 'bash fish zsh' -- "$current") )
      fi
      ;;
    list)
      if (( positional == 0 )); then
        COMPREPLY=( $(compgen -W '--running --remote --machine' -- "$current") )
      fi
      ;;
    start)
      candidates=(--attach --remote --machine)
      while IFS= read -r candidate; do
        [[ -n "$candidate" ]] && candidates+=("$candidate")
      done < <(tendr "${remote_args[@]}" __complete projects 2>/dev/null)
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
      if [[ "$current" == -* ]]; then
        COMPREPLY=( $(compgen -W '--remote --machine' -- "$current") )
        return
      fi
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
      done < <(tendr "${remote_args[@]}" __complete projects 2>/dev/null)
      ;;
  esac
}

complete -F _tendr tendr
`

const zshCompletion = `#compdef tendr

_tendr() {
  local command candidate remote machine
  local -i command_index i used positional
  local -a candidates projects sessions machines remote_args

  command=""
  command_index=-1
  remote=""
  machine=""
  positional=0
  if [[ "${words[CURRENT-1]}" == --machine ]]; then
    machines=("${(@f)$(tendr __complete machines 2>/dev/null)}")
    compadd -- "${machines[@]}"
    return
  fi
  [[ "${words[CURRENT-1]}" == --remote || "${words[CURRENT]}" == --remote=* || "${words[CURRENT]}" == --machine=* ]] && return

  for ((i = 2; i < CURRENT; i++)); do
    case "${words[i]}" in
      -d|--debug)
        ;;
      --remote)
        ((i++))
        remote="${words[i]}"
        ;;
      --remote=*)
        remote="${words[i]#--remote=}"
        ;;
      --machine)
        ((i++))
        machine="${words[i]}"
        ;;
      --machine=*)
        machine="${words[i]#--machine=}"
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
    candidates=(attach completion list start stop -d --debug --remote --machine -v --version -h --help)
    compadd -- "${candidates[@]}"
    return
  fi

  for ((i = command_index + 1; i < CURRENT; i++)); do
    case "${words[i]}" in
      --remote) ((i++)); remote="${words[i]}" ;;
      --remote=*) remote="${words[i]#--remote=}" ;;
      --machine) ((i++)); machine="${words[i]}" ;;
      --machine=*) machine="${words[i]#--machine=}" ;;
      --attach|--running) ;;
      *) ((positional++)) ;;
    esac
  done
  remote_args=()
  [[ -n "$remote" ]] && remote_args=(--remote "$remote")
  [[ -n "$machine" ]] && remote_args+=(--machine "$machine")

  case "$command" in
    attach)
      (( positional == 0 )) || return
      if [[ "${words[CURRENT]}" == -* ]]; then
        compadd -- --remote --machine
        return
      fi
      sessions=("${(@f)$(tendr "${remote_args[@]}" __complete sessions 2>/dev/null)}")
      compadd -- "${sessions[@]}"
      ;;
    completion)
      if (( CURRENT == command_index + 1 )); then
        compadd -- bash fish zsh
      fi
      ;;
    list)
      if (( positional == 0 )); then
        compadd -- --running --remote --machine
      fi
      ;;
    start)
      projects=(--attach --remote --machine "${(@f)$(tendr "${remote_args[@]}" __complete projects 2>/dev/null)}")
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
      if [[ "${words[CURRENT]}" == -* ]]; then
        compadd -- --remote --machine
        return
      fi
      projects=("${(@f)$(tendr "${remote_args[@]}" __complete projects 2>/dev/null)}")
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
function __tendr_tokens
    set -l skip_remote 0
    for token in (commandline -xpc)[2..-1]
        if test $skip_remote -eq 1
            set skip_remote 0
            continue
        end
        switch $token
            case --remote --machine
                set skip_remote 1
            case '--remote=*' '--machine=*'
            case '*'
                printf '%s\n' $token
        end
    end
end

function __tendr_query
    set -l tokens (commandline -xpc)
    set -l remote
    set -l machine
    for index in (seq 2 (count $tokens))
        if test "$tokens[$index]" = --remote
            set -l next (math $index + 1)
            if test $next -le (count $tokens)
                set remote $tokens[$next]
            end
        else if string match -q -- '--remote=*' $tokens[$index]
            set remote (string replace -- '--remote=' '' $tokens[$index])
        else if test "$tokens[$index]" = --machine
            set -l next (math $index + 1)
            if test $next -le (count $tokens)
                set machine $tokens[$next]
            end
        else if string match -q -- '--machine=*' $tokens[$index]
            set machine (string replace -- '--machine=' '' $tokens[$index])
        end
    end
    set -l target_args
    if test -n "$remote"
        set -a target_args --remote "$remote"
    end
    if test -n "$machine"
        set -a target_args --machine "$machine"
    end
    tendr $target_args $argv
end

function __tendr_needs_target
    set -l tokens (commandline -xpc)
    contains -- "$tokens[-1]" --remote --machine
end

function __tendr_needs_machine
    set -l tokens (commandline -xpc)
    test "$tokens[-1]" = --machine
end

function __tendr_machines
    for machine in (tendr __complete machines 2>/dev/null)
        string escape -- $machine
    end
end

function __tendr_no_subcommand
    __tendr_needs_target; and return 1
    for token in (__tendr_tokens)
        switch $token
            case -v --version -h --help attach completion list start stop
                return 1
        end
    end
    return 0
end

function __tendr_using_subcommand
    __tendr_needs_target; and return 1
    for token in (__tendr_tokens)
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
    set -l tokens (__tendr_tokens)
    set -l command_index (contains -i -- $argv[1] $tokens)
    test -n "$command_index"; and test (count $tokens) -eq $command_index
end

function __tendr_projects
    set -l used (commandline -xpc)
    for project in (__tendr_query __complete projects 2>/dev/null)
        if not contains -- $project $used
            string escape -- $project
        end
    end
end

function __tendr_running_sessions
    for session in (__tendr_query __complete sessions 2>/dev/null)
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
complete -c tendr -n '__tendr_no_subcommand; or __tendr_using_subcommand start; or __tendr_using_subcommand attach; or __tendr_using_subcommand list; or __tendr_using_subcommand stop' -l remote -r -d 'SSH target'
complete -c tendr -n '__tendr_needs_machine; or __tendr_no_subcommand; or __tendr_using_subcommand start; or __tendr_using_subcommand attach; or __tendr_using_subcommand list; or __tendr_using_subcommand stop' -l machine -r -a '(__tendr_machines)' -d 'Saved Herdr machine'

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
