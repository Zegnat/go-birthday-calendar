// SPDX-FileCopyrightText: 2026 Martijn van der Ven
// SPDX-License-Identifier: 0BSD

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeriveName(t *testing.T) {
	tests := map[string]string{
		"-":               "birthdays",
		"family.txt":      "family",
		"path/family.txt": "family",
		"family":          "family",
		"family.tar.gz":   "family.tar",
	}
	for in, want := range tests {
		if got := deriveName(in); got != want {
			t.Errorf("deriveName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildVersionOverride(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v1.2.3"
	if got := buildVersion(); got != "v1.2.3" {
		t.Errorf("buildVersion() = %q, want %q", got, "v1.2.3")
	}
}

func TestBuildVersionFallback(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = ""
	// buildVersion may return debug.ReadBuildInfo's Main.Version or
	// "dev" — both are non-empty.
	if got := buildVersion(); got == "" {
		t.Error("buildVersion() returned empty string")
	}
}

// runCase exercises the run() entry point with a given argv. It
// returns the exit code along with whatever was written to stdout,
// stderr, and (when output goes to a file) that file's contents.
type runCase struct {
	args      []string
	stdinData string

	// outFile, when set, is the absolute path expected to receive the
	// ICS output. The test reads it back into wantOut.
	outFile string
}

type runResult struct {
	code   int
	stdout string
	stderr string
	out    []byte
}

func runWithArgs(t *testing.T, c runCase) runResult {
	t.Helper()

	oldArgs := os.Args
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	os.Args = append([]string{"birthday-calendar"}, c.args...)

	if c.stdinData != "" {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("stdin pipe: %v", err)
		}
		os.Stdin = r
		go func() {
			defer func() { _ = w.Close() }()
			_, _ = io.WriteString(w, c.stdinData)
		}()
	}

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	code := run()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = io.Copy(&stdoutBuf, stdoutR)
	_, _ = io.Copy(&stderrBuf, stderrR)

	res := runResult{
		code:   code,
		stdout: stdoutBuf.String(),
		stderr: stderrBuf.String(),
	}
	if c.outFile != "" {
		data, err := os.ReadFile(c.outFile)
		if err == nil {
			res.out = data
		}
	}
	return res
}

func writeFixture(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func TestRunHelpFlag(t *testing.T) {
	res := runWithArgs(t, runCase{args: []string{"--help"}})
	if res.code != 0 {
		t.Errorf("--help exit = %d, want 0", res.code)
	}
}

func TestRunVersionFlag(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "v1.2.3-test"
	res := runWithArgs(t, runCase{args: []string{"--version"}})
	if res.code != 0 {
		t.Errorf("--version exit = %d, want 0", res.code)
	}
	if !strings.Contains(res.stdout, "v1.2.3-test") {
		t.Errorf("--version stdout = %q, want it to contain version", res.stdout)
	}
}

func TestRunNegativeUniquePast(t *testing.T) {
	res := runWithArgs(t, runCase{args: []string{"-p", "-1"}})
	if res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.stderr, "non-negative") {
		t.Errorf("stderr = %q, want non-negative complaint", res.stderr)
	}
}

func TestRunInvalidNow(t *testing.T) {
	res := runWithArgs(t, runCase{args: []string{"--now", "not-a-date"}})
	if res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.stderr, "--now") {
		t.Errorf("stderr = %q, want --now complaint", res.stderr)
	}
}

func TestRunBadTitleTemplate(t *testing.T) {
	res := runWithArgs(t, runCase{args: []string{"-t", "{{.UnterminatedAction"}})
	if res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.stderr, "--event-title") {
		t.Errorf("stderr = %q, want --event-title complaint", res.stderr)
	}
}

func TestRunBadDescriptionTemplate(t *testing.T) {
	res := runWithArgs(t, runCase{args: []string{"-d", "{{.Bad"}})
	if res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.stderr, "--event-description") {
		t.Errorf("stderr = %q, want --event-description complaint", res.stderr)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	res := runWithArgs(t, runCase{args: []string{"--no-such-flag"}})
	if res.code != 1 {
		t.Errorf("exit = %d, want 1", res.code)
	}
}

func TestRunMissingInputFile(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist.txt")
	res := runWithArgs(t, runCase{args: []string{"-i", missing}})
	if res.code != 2 {
		t.Errorf("exit = %d, want 2", res.code)
	}
}

func TestRunOutputDirMissing(t *testing.T) {
	tmp := t.TempDir()
	input := writeFixture(t, tmp, "in.txt", "1990-03-15 Alice\n")
	bad := filepath.Join(tmp, "no", "such", "dir", "out.ics")
	res := runWithArgs(t, runCase{args: []string{"-i", input, "-o", bad}})
	if res.code != 2 {
		t.Errorf("exit = %d, want 2", res.code)
	}
}

func TestRunInvalidInputData(t *testing.T) {
	tmp := t.TempDir()
	input := writeFixture(t, tmp, "bad.txt", "1990-13-01 NopeMonth\n")
	out := filepath.Join(tmp, "out.ics")
	res := runWithArgs(t, runCase{
		args:    []string{"-i", input, "-o", out, "--now", "2026-06-01"},
		outFile: out,
	})
	if res.code != 2 {
		t.Errorf("exit = %d, want 2", res.code)
	}
}

func TestRunFileToFile(t *testing.T) {
	tmp := t.TempDir()
	input := writeFixture(t, tmp, "family.txt", "1990-03-15 Alice\n")
	out := filepath.Join(tmp, "family.ics")
	res := runWithArgs(t, runCase{
		args: []string{
			"-i", input, "-o", out,
			"--now", "2026-06-01",
			"--prodid", "-//test//EN",
			"--uid-domain", "example.invalid",
			"-t", "Title: {{.Name}}",
			"-d", "Desc: {{.Name}}",
		},
		outFile: out,
	})
	if res.code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", res.code, res.stderr)
	}
	body := strings.ReplaceAll(string(res.out), "\r\n ", "")
	for _, want := range []string{
		"BEGIN:VCALENDAR",
		"END:VCALENDAR",
		"X-WR-CALNAME:family",
		"PRODID:-//test//EN",
		"SUMMARY:Title: Alice",
		"DESCRIPTION:Desc: Alice",
		"@example.invalid",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("output missing %q\n%s", want, body)
		}
	}
}

func TestRunStdinToStdout(t *testing.T) {
	res := runWithArgs(t, runCase{
		args:      []string{"--now", "2026-06-01", "--prodid", "-//test//EN"},
		stdinData: "1990-03-15 Alice\n",
	})
	if res.code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "X-WR-CALNAME:birthdays") {
		t.Errorf("stdout missing derived stdin calendar name:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "SUMMARY:Alice (1990)") {
		t.Errorf("stdout missing default-formatted summary:\n%s", res.stdout)
	}
}

func TestRunDefaultPRODID(t *testing.T) {
	tmp := t.TempDir()
	input := writeFixture(t, tmp, "in.txt", "1990-03-15 Alice\n")
	out := filepath.Join(tmp, "out.ics")
	old := version
	t.Cleanup(func() { version = old })
	version = "v7.7.7"
	res := runWithArgs(t, runCase{
		args:    []string{"-i", input, "-o", out, "--now", "2026-06-01"},
		outFile: out,
	})
	if res.code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(string(res.out), "v7.7.7") {
		t.Errorf("default PRODID did not embed version:\n%s", res.out)
	}
}
