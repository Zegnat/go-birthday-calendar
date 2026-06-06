// SPDX-FileCopyrightText: 2026 Martijn van der Ven
// SPDX-License-Identifier: 0BSD

package birthdaycal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"
	"unicode/utf8"
)

var testNow = time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

func TestGenerateDifferentCalNames(t *testing.T) {
	uidFor := func(calName string) string {
		var buf bytes.Buffer
		err := Generate(strings.NewReader("0000-01-01 Alice\n"), &buf, Options{
			CalendarName: calName,
			Now:          testNow,
		})
		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}
		for line := range strings.SplitSeq(buf.String(), "\r\n") {
			if strings.HasPrefix(line, "UID:") {
				return line
			}
		}
		t.Fatal("no UID line in output")
		return ""
	}
	if uidFor("family") == uidFor("friends") {
		t.Error("different calendar names should produce different UIDs")
	}
}

func TestGenerateInvalidInput(t *testing.T) {
	var buf bytes.Buffer
	err := Generate(strings.NewReader("not-a-date Oops\n"), &buf, Options{
		CalendarName: "test",
		Now:          testNow,
	})
	if err == nil {
		t.Fatal("expected error for invalid input, got nil")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Line != 1 {
		t.Errorf("expected error on line 1, got %d", pe.Line)
	}
}

func TestGenerateLineTooLong(t *testing.T) {
	input := "1990-03-15 Alice\n" + strings.Repeat("A", maxInputLineBytes+1) + "\n"
	var buf bytes.Buffer
	err := Generate(strings.NewReader(input), &buf, Options{Now: testNow})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
	if pe.Line != 2 {
		t.Errorf("ParseError.Line = %d, want 2", pe.Line)
	}
	if !strings.Contains(pe.Message, "exceeds") {
		t.Errorf("ParseError.Message = %q, want message about exceeded size", pe.Message)
	}
}

// TestGenerateUsesTimeNowWhenZero covers the applyDefaults branch that
// fills opts.Now with time.Now() when callers leave it zero. testGenerate
// pre-fills Now, so no fixture or other test exercises this path.
func TestGenerateUsesTimeNowWhenZero(t *testing.T) {
	if err := Generate(strings.NewReader(""), io.Discard, Options{}); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
}

func TestGenerateUIDFieldsAreDistinct(t *testing.T) {
	a := generateUID("d", "cal", "Alice/2023")
	b := generateUID("d", "cal", "Alice", "/2023")
	if a == b {
		t.Errorf("UID for (Alice/2023, no suffix) and (Alice, /2023) should differ, both = %s", a)
	}
}

func TestWriteFolded(t *testing.T) {
	iw := &icalWriter{w: &bytes.Buffer{}}
	long := "SUMMARY:" + strings.Repeat("A", 100)
	iw.writeFolded(long)
	for line := range strings.SplitSeq(iw.w.(*bytes.Buffer).String(), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line exceeds 75 octets: %d chars", len(line))
		}
	}
}

func TestWriteFoldedUTF8(t *testing.T) {
	// Place a multi-byte rune (é = 0xC3 0xA9) so the 75-byte boundary
	// lands on its continuation byte. Without rune-aligned folding the
	// first chunk would end with the stray lead byte 0xC3.
	prefix := "SUMMARY:" + strings.Repeat("A", 66)
	line := prefix + "é" + "tail-after-fold"
	if !utf8.ValidString(line) {
		t.Fatalf("test input not valid UTF-8")
	}
	var buf bytes.Buffer
	iw := &icalWriter{w: &buf}
	iw.writeFolded(line)
	out := strings.TrimSuffix(buf.String(), "\r\n")
	chunks := strings.Split(out, "\r\n")
	if len(chunks) < 2 {
		t.Fatalf("expected the line to be folded, got %d chunk(s):\n%s", len(chunks), out)
	}
	for i, c := range chunks {
		if !utf8.ValidString(c) {
			t.Errorf("chunk %d not valid UTF-8: %q", i, c)
		}
		if len(c) > 75 {
			t.Errorf("chunk %d exceeds 75 octets (%d): %q", i, len(c), c)
		}
	}
	var unfolded strings.Builder
	for i, c := range chunks {
		if i > 0 {
			c = strings.TrimPrefix(c, " ")
		}
		unfolded.WriteString(c)
	}
	if unfolded.String() != line {
		t.Errorf("unfold mismatch:\n got %q\nwant %q", unfolded.String(), line)
	}
}

func TestICalEscapeText(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"plain", "plain"},
		{`a\b`, `a\\b`},
		{"a;b", `a\;b`},
		{"a,b", `a\,b`},
		{"a\nb", `a\nb`},
		{"a\r\nb", `a\nb`},
		{"a\rb", `a\nb`},
		{`one\ two; three, four` + "\nfive", `one\\ two\; three\, four\nfive`},
	}
	for _, tc := range tests {
		if got := icalEscapeText(tc.in); got != tc.want {
			t.Errorf("icalEscapeText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// failingWriter returns the configured error after `okBytes` successful
// bytes have been written. Used to exercise error paths in icalWriter.
type failingWriter struct {
	okBytes int
	err     error
}

func (f *failingWriter) Write(p []byte) (int, error) {
	if f.okBytes <= 0 {
		return 0, f.err
	}
	if len(p) <= f.okBytes {
		f.okBytes -= len(p)
		return len(p), nil
	}
	n := f.okBytes
	f.okBytes = 0
	return n, f.err
}

func TestIcalWriterShortCircuitsOnPriorError(t *testing.T) {
	sentinel := errors.New("prior")
	var buf bytes.Buffer
	iw := &icalWriter{w: &buf, err: sentinel}
	iw.writeLine("X")
	iw.writeFolded("Y")
	if buf.Len() != 0 {
		t.Errorf("writeLine/writeFolded wrote %d bytes despite prior error", buf.Len())
	}
	if iw.err != sentinel {
		t.Errorf("err = %v, want %v", iw.err, sentinel)
	}
}

func TestWriteFoldedWriteError(t *testing.T) {
	wantErr := errors.New("write boom")
	iw := &icalWriter{w: &failingWriter{okBytes: 0, err: wantErr}}
	iw.writeFolded(strings.Repeat("A", 200))
	if iw.err != wantErr {
		t.Errorf("err = %v, want %v", iw.err, wantErr)
	}
}

func TestWriteFoldedWriteErrorMidFold(t *testing.T) {
	wantErr := errors.New("second-chunk boom")
	iw := &icalWriter{w: &failingWriter{okBytes: 77, err: wantErr}}
	iw.writeFolded(strings.Repeat("A", 200))
	if iw.err != wantErr {
		t.Errorf("err = %v, want %v", iw.err, wantErr)
	}
}

func TestGenerateTemplateError(t *testing.T) {
	// Always errors at Execute time.
	always := template.Must(template.New("t").Option("missingkey=error").Parse(`{{.NonExistent}}`))
	// Errors only when .Age is non-empty (i.e. on per-year unique events,
	// not on recurring events).
	onAge := template.Must(template.New("t").Option("missingkey=error").Parse(`{{if .Age}}{{.NonExistent}}{{end}}`))

	tests := []struct {
		name                     string
		title, desc              *template.Template
		uniquePast, uniqueFuture int
	}{
		{name: "title always, no window", title: always},
		{name: "desc always, no window", desc: always},
		{name: "title always, with window", title: always, uniquePast: 1, uniqueFuture: 1},
		{name: "desc always, with window", desc: always, uniquePast: 1, uniqueFuture: 1},
		{name: "title fails only on age", title: onAge, uniquePast: 1, uniqueFuture: 1},
		{name: "desc fails only on age", desc: onAge, uniquePast: 1, uniqueFuture: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Generate(strings.NewReader("1990-03-15 Alice\n"), io.Discard, Options{
				Title:        tc.title,
				Description:  tc.desc,
				Now:          testNow,
				UniquePast:   tc.uniquePast,
				UniqueFuture: tc.uniqueFuture,
			})
			if err == nil {
				t.Fatal("expected error from failing template")
			}
		})
	}
}

// erroringReader returns its error after one successful read.
type erroringReader struct {
	once bool
	err  error
}

func (r *erroringReader) Read(p []byte) (int, error) {
	if !r.once {
		r.once = true
		copy(p, "1990-03-15 Alice")
		return len("1990-03-15 Alice"), nil
	}
	return 0, r.err
}

func TestGenerateScannerError(t *testing.T) {
	wantErr := errors.New("read fail")
	err := Generate(&erroringReader{err: wantErr}, io.Discard, Options{Now: testNow})
	if err != wantErr {
		t.Errorf("Generate returned %v, want %v", err, wantErr)
	}
}

// fixtureOptions is the JSON shape of testdata/<case>/options.json.
type fixtureOptions struct {
	Name         string `json:"name,omitempty"`
	Now          string `json:"now,omitempty"`
	UniquePast   int    `json:"unique_past,omitempty"`
	UniqueFuture int    `json:"unique_future,omitempty"`
	PRODID       string `json:"prodid,omitempty"`
	UIDDomain    string `json:"uid_domain,omitempty"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
}

// TestGenerateFixtures walks testdata/, runs Generate against each case's
// input.txt + options.json, and compares the result byte-for-byte against
// output.ics. The expected output.ics is hand-authored (typically by
// piping the CLI's output once and reviewing it).
func TestGenerateFixtures(t *testing.T) {
	root := "testdata"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			base := filepath.Join(root, name)

			input, err := os.ReadFile(filepath.Join(base, "input.txt"))
			if err != nil {
				t.Skipf("no input.txt in %s: %v", base, err)
			}

			var fix fixtureOptions
			if raw, err := os.ReadFile(filepath.Join(base, "options.json")); err == nil {
				if err := json.Unmarshal(raw, &fix); err != nil {
					t.Fatalf("parse options.json: %v", err)
				}
			}

			opts := Options{
				CalendarName: fix.Name,
				UniquePast:   fix.UniquePast,
				UniqueFuture: fix.UniqueFuture,
				PRODID:       fix.PRODID,
				UIDDomain:    fix.UIDDomain,
			}
			if fix.Now != "" {
				now, err := time.Parse("2006-01-02", fix.Now)
				if err != nil {
					t.Fatalf("parse now: %v", err)
				}
				opts.Now = now
			} else {
				opts.Now = testNow
			}
			if fix.Title != "" {
				opts.Title = template.Must(template.New("title").Parse(fix.Title))
			}
			if fix.Description != "" {
				opts.Description = template.Must(template.New("desc").Parse(fix.Description))
			}

			var buf bytes.Buffer
			if err := Generate(bytes.NewReader(input), &buf, opts); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			want, err := os.ReadFile(filepath.Join(base, "output.ics"))
			if err != nil {
				t.Fatalf("read output.ics: %v", err)
			}
			if !bytes.Equal(buf.Bytes(), want) {
				t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, buf.String(), string(want))
			}
		})
	}
}
