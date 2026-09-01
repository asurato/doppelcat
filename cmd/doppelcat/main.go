package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"github.com/asurato/doppelcat/internal/document"
	"github.com/asurato/doppelcat/internal/ui"
)

// version can be set with -ldflags. When installed with `go install module@version`,
// the module version embedded by Go is used instead.
var version = "dev"

func appVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

const help = `Usage: doppelcat [options] <file>

Watch, diff, and lightly edit one UTF-8 text file.

Viewing: arrows scroll, Ctrl+Home/End jump, d toggles diff, e edits, q quits
Editing: arrows/Home/End, Shift selects, Ctrl+C/X/V, Ctrl+Z/Y, Ctrl+S saves,
         Esc leaves edit mode, Ctrl+Q quits from any mode

Options:
  --update-delay milliseconds  wait after the last change before updating (default 200)
  --help                       show this help
  --version                    show version
`

func main() { os.Exit(run(os.Args[1:])) }

func parseRunArgs(args []string) (string, time.Duration, error) {
	flags := flag.NewFlagSet("doppelcat", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	updateDelay := flags.Int("update-delay", int(ui.DefaultDebounce/time.Millisecond), "")
	if err := flags.Parse(args); err != nil {
		return "", 0, err
	}
	if flags.NArg() != 1 {
		return "", 0, fmt.Errorf("expected exactly one file")
	}
	if *updateDelay <= 0 {
		return "", 0, fmt.Errorf("--update-delay must be greater than zero")
	}
	return flags.Arg(0), time.Duration(*updateDelay) * time.Millisecond, nil
}

func run(args []string) int {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Print(help)
		return 0
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println(appVersion())
		return 0
	}
	path, debounce, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doppelcat: %v (try --help)\n", err)
		return 2
	}
	s, err := document.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doppelcat: %v\n", err)
		return 1
	}
	if err = ui.RunWithDebounce(path, s, debounce); err != nil {
		fmt.Fprintf(os.Stderr, "doppelcat: %v\n", err)
		return 1
	}
	return 0
}
