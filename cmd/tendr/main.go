package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	tendrcmd "github.com/tombell/tendr/internal/cmd"
)

const helpText = `usage: tendr [<flags>] <command>

Commands:

  list          List configured projects
  start         Start Herdr project sessions
  stop          Stop Herdr project sessions

Special options:

  -d/--debug    Show debug logging
  -v/--version  Show the version number, then exit
  --help        Show this message, then exit
`

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		log.SetFlags(0)
		log.Fatal(err)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
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

	app := tendrcmd.New(logger, stdout)
	switch remaining[0] {
	case "list":
		return app.List()
	case "start":
		return app.Start(remaining[1:])
	case "stop":
		return app.Stop(remaining[1:])
	default:
		return fmt.Errorf("%q is not a known command", remaining[0])
	}
}
