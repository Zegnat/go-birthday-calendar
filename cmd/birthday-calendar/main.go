// SPDX-FileCopyrightText: 2026 Martijn van der Ven
// SPDX-License-Identifier: 0BSD

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"text/template"
	"time"

	birthdaycal "github.com/Zegnat/go-birthday-calendar"
	pflag "github.com/spf13/pflag"
)

func main() { // coverage-ignore: thin wrapper around run() that calls os.Exit, untestable from `go test`.
	os.Exit(run())
}

func run() int {
	flags := pflag.NewFlagSet("birthday-calendar", pflag.ContinueOnError)

	input := flags.StringP("input", "i", "-", "input birthday file (\"-\" for stdin)")
	output := flags.StringP("output", "o", "-", "output iCalendar file (\"-\" for stdout)")
	name := flags.StringP("name", "n", "", "calendar name (default: derived from input filename)")
	uniquePast := flags.IntP("unique-past", "p", 0, "years before --now with individual age events")
	uniqueFuture := flags.IntP("unique-future", "f", 0, "years after --now with individual age events")
	titleTmpl := flags.StringP("event-title", "t", "", "Go template for event title")
	descTmpl := flags.StringP("event-description", "d", "", "Go template for event description")
	prodid := flags.String("prodid", "", "iCalendar PRODID")
	uidDomain := flags.String("uid-domain", "", "domain for the right-hand side of each VEVENT UID")
	nowStr := flags.String("now", "", "reference date (YYYY-MM-DD)")
	version := flags.BoolP("version", "v", false, "print version and exit")

	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "birthday-calendar transforms a text list of birthdays into an iCalendar (.ics) file.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  birthday-calendar [options] [-i FILE] [-o FILE]")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  birthday-calendar -i family.txt -o family.ics -p 3 -f 5")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		flags.PrintDefaults()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Default --event-title template:")
		fmt.Fprintln(os.Stderr, "  "+birthdaycal.DefaultTitleTemplate)
		fmt.Fprintln(os.Stderr, "Default --event-description template:")
		fmt.Fprintln(os.Stderr, "  "+birthdaycal.DefaultDescriptionTemplate)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Exit codes:")
		fmt.Fprintln(os.Stderr, "  0  success")
		fmt.Fprintln(os.Stderr, "  1  invalid command-line arguments")
		fmt.Fprintln(os.Stderr, "  2  I/O, parse, or template error")
	}

	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == pflag.ErrHelp {
			return 0
		}
		return 1
	}

	if *version {
		printVersion()
		return 0
	}

	if *uniquePast < 0 || *uniqueFuture < 0 {
		fmt.Fprintf(os.Stderr, "error: --unique-past and --unique-future must be non-negative\n")
		return 1
	}

	calName := *name
	if calName == "" {
		calName = deriveName(*input)
	}

	var now time.Time
	if *nowStr != "" {
		var err error
		now, err = time.Parse("2006-01-02", *nowStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid --now value %q: expected YYYY-MM-DD\n", *nowStr)
			return 1
		}
	}

	pid := *prodid
	if pid == "" {
		pid = defaultPRODID()
	}

	opts := birthdaycal.Options{
		CalendarName: calName,
		Now:          now,
		UniquePast:   *uniquePast,
		UniqueFuture: *uniqueFuture,
		PRODID:       pid,
		UIDDomain:    *uidDomain,
	}

	if *titleTmpl != "" {
		t, err := template.New("title").Parse(*titleTmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid --event-title template: %v\n", err)
			return 1
		}
		opts.Title = t
	}

	if *descTmpl != "" {
		t, err := template.New("description").Parse(*descTmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid --event-description template: %v\n", err)
			return 1
		}
		opts.Description = t
	}

	r, closeR, err := openInput(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer closeR()

	w, closeW, err := openOutput(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}
	defer closeW()

	if err := birthdaycal.Generate(r, w, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	return 0
}

func openInput(path string) (*os.File, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}

func openOutput(path string) (*os.File, func(), error) {
	if path == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { _ = f.Close() }, nil
}

func deriveName(input string) string {
	if input == "-" {
		return "birthdays"
	}
	base := filepath.Base(input)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

var version string

func buildVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" { // coverage-ignore: `go test` binaries always carry a non-empty info.Main.Version.
		return "dev"
	}
	return info.Main.Version
}

func printVersion() {
	fmt.Printf("birthday-calendar %s\n", buildVersion())
}

func defaultPRODID() string {
	return fmt.Sprintf("+//IDN zegnat.net//NONSGML birthdaycal %s//EN", buildVersion())
}
