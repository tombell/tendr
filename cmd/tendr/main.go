package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	tendrcmd "github.com/tombell/tendr/internal/cmd"
)

const helpText = `usage: tendr [<flags>] <command>

Commands:

  attach        Attach to a Herdr session
  completion    Generate shell completion script
  list          List configured projects
  start         Start Herdr project sessions
  stop          Stop Herdr project sessions

Special options:

  -d/--debug    Show debug logging
  -v/--version  Show the version number, then exit
  --help        Show this message, then exit
`

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		log.SetFlags(0)
		log.Fatal(err)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("tendr", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, helpText) }

	var debug bool
	var version bool
	flags.BoolVar(&debug, "debug", false, "")
	flags.BoolVar(&debug, "d", false, "")
	flags.BoolVar(&version, "version", false, "")
	flags.BoolVar(&version, "v", false, "")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if version {
		fmt.Fprintf(stdout, "tendr %s (%s)\n", Version, Commit)
		return nil
	}

	remaining := flags.Args()
	if len(remaining) == 0 {
		flags.Usage()
		return flag.ErrHelp
	}

	var logger *log.Logger
	if debug {
		logger = log.New(stderr, "", 0)
	}

	app := tendrcmd.New(logger, stdin, stdout, stderr)
	switch remaining[0] {
	case "__complete":
		if len(remaining) != 2 || remaining[1] != "sessions" {
			return errors.New("usage: tendr __complete sessions")
		}
		return app.ListRunningSessions()
	case "attach":
		if len(remaining) != 2 {
			return errors.New("usage: tendr attach <name>")
		}
		return app.Attach(remaining[1])
	case "completion":
		if len(remaining) != 2 {
			return errors.New("usage: tendr completion <bash|zsh>")
		}
		return app.Completion(remaining[1])
	case "list":
		if len(remaining) != 1 {
			return errors.New("usage: tendr list")
		}
		return app.List()
	case "start":
		return app.Start(remaining[1:])
	case "stop":
		return app.Stop(remaining[1:])
	default:
		return fmt.Errorf("%q is not a known command", remaining[0])
	}
}
