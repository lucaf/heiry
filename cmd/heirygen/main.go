// Command heirygen writes heiry declarations, initialization helpers,
// operation interfaces, and public wrappers from specifications held in
// marked Go string constants. Generated code uses only the heiry public API.
//
// Usage:
//
//	heirygen [-check] [-v] [dir]
//	heirygen -version
//
// heirygen reads the Go files of dir, the current directory by default, and
// writes one generated file next to each file that holds a specification.
// With -check it writes nothing and reports outputs that are not up to date.
//
// Exit status: 0 on success, 1 when -check finds outputs that are not up to
// date, 2 for a usage error, and 3 when generation or checking fails.
//
// See docs/user/generation.md in the heiry repository.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/lucaf/heiry/cmd/heirygen/internal/generator"
)

const (
	exitOK = iota
	exitStale
	exitUsage
	exitFailed
)

const usage = "usage: heirygen [-check] [-v] [dir]\n       heirygen -version\n"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("heirygen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, usage)
		flags.PrintDefaults()
	}
	check := flags.Bool("check", false, "report outputs that are not up to date, without writing")
	verbose := flags.Bool("v", false, "list the files written or removed")
	showVersion := flags.Bool("version", false, "print the module and version the command was built from")
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if *showVersion {
		if flags.NArg() != 0 || *check || *verbose {
			fmt.Fprint(stderr, usage)
			return exitUsage
		}
		fmt.Fprintln(stdout, version())
		return exitOK
	}
	if flags.NArg() > 1 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	dir := "."
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}
	if *check {
		findings, err := generator.Check(dir)
		if err != nil {
			fmt.Fprintln(stderr, "heirygen:", err)
			return exitFailed
		}
		for _, finding := range findings {
			fmt.Fprintln(stderr, finding)
		}
		if len(findings) != 0 {
			return exitStale
		}
		return exitOK
	}
	report, err := generator.Generate(dir)
	if *verbose {
		for _, name := range report.Written {
			fmt.Fprintln(stdout, "wrote", name)
		}
		for _, name := range report.Removed {
			fmt.Fprintln(stdout, "removed", name)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "heirygen:", err)
		return exitFailed
	}
	return exitOK
}

// version names the heiry module the command was built from. A consumer's
// go.mod selects it, so the generator and the library share one version.
func version() string {
	const module = "github.com/lucaf/heiry"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "heirygen " + module + " (unknown version)"
	}
	found := &info.Main
	if found.Path != module {
		found = nil
		for _, dep := range info.Deps {
			if dep.Path == module {
				found = dep
			}
		}
	}
	if found == nil {
		return "heirygen " + module + " (unknown version)"
	}
	text := "heirygen " + module + " " + found.Version
	if found.Replace != nil {
		text += " => " + found.Replace.Path
		if found.Replace.Version != "" {
			text += " " + found.Replace.Version
		}
	}
	return text
}
