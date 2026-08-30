package main

import (
	"fmt"
	"os"
	"runtime/debug"

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

const help = `Usage: doppelcat <file>

Watch, diff, and lightly edit one UTF-8 text file.

Viewing: arrows scroll, Ctrl+Home/End jump, d toggles diff, e edits, q quits
Editing: arrows/Home/End, Shift selects, Ctrl+C/X/V, Ctrl+Z/Y, Ctrl+S saves,
         Esc leaves edit mode, Ctrl+Q quits from any mode

Options:
  --help       show this help
  --version    show version
`

func main() { os.Exit(run(os.Args[1:])) }
func run(args []string) int {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Print(help)
		return 0
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println(appVersion())
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: doppelcat <file> (try --help)")
		return 2
	}
	s, err := document.Load(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "doppelcat: %v\n", err)
		return 1
	}
	if err = ui.Run(args[0], s); err != nil {
		fmt.Fprintf(os.Stderr, "doppelcat: %v\n", err)
		return 1
	}
	return 0
}
