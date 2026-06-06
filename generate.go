// SPDX-FileCopyrightText: 2026 Martijn van der Ven
// SPDX-License-Identifier: 0BSD

package birthdaycal

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"
)

// maxInputLineBytes bounds the size of a single input line. The format
// is "YYYY-MM-DD Name", so a line of more than a megabyte is either a
// mistake or an adversarial input.
const maxInputLineBytes = 1 << 20

var (
	defaultTitle       = template.Must(template.New("title").Parse(DefaultTitleTemplate))
	defaultDescription = template.Must(template.New("description").Parse(DefaultDescriptionTemplate))
)

// Generate reads birthdays from r — one "YYYY-MM-DD Name" entry per
// line, with blank lines and lines beginning with '#' ignored — and
// writes an iCalendar (RFC 5545) stream to w containing one VCALENDAR
// with a VEVENT per birthday.
//
// Input-format problems are returned as a [*ParseError] whose Line
// field identifies the offending line. Other failures (template
// execution, write errors, scanner errors) are returned as-is.
//
// The zero value of opts is valid; see [Options] for the fields that
// influence the output.
func Generate(r io.Reader, w io.Writer, opts Options) error {
	opts = applyDefaults(opts)

	iw := &icalWriter{w: w}
	iw.writeLine("BEGIN:VCALENDAR")
	iw.writeLine("VERSION:2.0")
	iw.writeFolded("PRODID:" + icalEscapeText(opts.PRODID))
	iw.writeFolded("X-WR-CALNAME:" + icalEscapeText(opts.CalendarName))

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxInputLineBytes)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		bd, err := parseLine(line)
		if err != nil {
			return &ParseError{Line: lineNum, Message: err.Error()}
		}

		if err := writeEvents(iw, opts, bd); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return &ParseError{
				Line:    lineNum + 1,
				Message: fmt.Sprintf("line exceeds %d bytes", maxInputLineBytes),
			}
		}
		return err
	}

	iw.writeLine("END:VCALENDAR")
	return iw.err
}

func applyDefaults(opts Options) Options {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.PRODID == "" {
		opts.PRODID = "-//go-birthday-calendar//NONSGML birthdaycal//EN"
	}
	if opts.UIDDomain == "" {
		opts.UIDDomain = "birthday-calendar"
	}
	if opts.Title == nil {
		opts.Title = defaultTitle
	}
	if opts.Description == nil {
		opts.Description = defaultDescription
	}
	return opts
}

func writeEvents(iw *icalWriter, opts Options, bd Birthday) error {
	hasWindow := opts.UniquePast > 0 || opts.UniqueFuture > 0

	if bd.Year == nil || !hasWindow {
		return writeRecurringEvent(iw, opts, bd, recurringStartYear(bd), "", "")
	}

	currentYear := opts.Now.Year()
	windowStart := currentYear - opts.UniquePast
	windowEnd := currentYear + opts.UniqueFuture

	untilYear := windowStart - 1
	untilDay := bd.Day
	if bd.Month == time.February && bd.Day == 29 && !isLeapYear(untilYear) {
		untilDay = 28
	}
	until := fmt.Sprintf(";UNTIL=%04d%02d%02d", untilYear, bd.Month, untilDay)
	if err := writeRecurringEvent(iw, opts, bd, *bd.Year, "/early", until); err != nil {
		return err
	}

	for y := windowStart; y <= windowEnd; y++ {
		age := y - *bd.Year
		if err := writeUniqueEvent(iw, opts, bd, y, age); err != nil {
			return err
		}
	}

	return writeRecurringEvent(iw, opts, bd, windowEnd+1, "/late", "")
}

// unknownYearFallback is used as DTSTART year when a birthday's year is
// not known. It must be a leap year so that Feb 29 birthdays produce a
// valid DTSTART; 2000 is the most recent year divisible by 400.
const unknownYearFallback = 2000

func recurringStartYear(bd Birthday) int {
	if bd.Year != nil {
		return *bd.Year
	}
	return unknownYearFallback
}

func writeRecurringEvent(iw *icalWriter, opts Options, bd Birthday, startYear int, uidSuffix string, rruleExtra string) error {
	data := EventData{
		Name:      bd.Name,
		BirthDate: Date{Month: bd.Month, Day: bd.Day, Year: bd.Year},
	}

	summary, err := renderTemplate(opts.Title, data)
	if err != nil {
		return err
	}
	description, err := renderTemplate(opts.Description, data)
	if err != nil {
		return err
	}

	uid := generateUID(opts.UIDDomain, opts.CalendarName, bd.Name, uidSuffix)
	dtstart := fmt.Sprintf("%04d%02d%02d", startYear, bd.Month, bd.Day)

	iw.writeLine("BEGIN:VEVENT")
	iw.writeFolded("UID:" + uid)
	iw.writeLine("DTSTART;VALUE=DATE:" + dtstart)
	iw.writeLine("RRULE:FREQ=YEARLY" + rruleExtra)
	iw.writeFolded("SUMMARY:" + icalEscapeText(summary))
	if description != "" {
		iw.writeFolded("DESCRIPTION:" + icalEscapeText(description))
	}
	iw.writeLine("TRANSP:TRANSPARENT")
	iw.writeLine("END:VEVENT")
	return iw.err
}

func writeUniqueEvent(iw *icalWriter, opts Options, bd Birthday, year int, age int) error {
	data := EventData{
		Name:      bd.Name,
		BirthDate: Date{Month: bd.Month, Day: bd.Day, Year: bd.Year},
		Age:       fmt.Sprintf("%d", age),
	}

	summary, err := renderTemplate(opts.Title, data)
	if err != nil {
		return err
	}
	description, err := renderTemplate(opts.Description, data)
	if err != nil {
		return err
	}

	uid := generateUID(opts.UIDDomain, opts.CalendarName, bd.Name, fmt.Sprintf("/%d", year))
	dtstart := fmt.Sprintf("%04d%02d%02d", year, bd.Month, bd.Day)

	iw.writeLine("BEGIN:VEVENT")
	iw.writeFolded("UID:" + uid)
	iw.writeLine("DTSTART;VALUE=DATE:" + dtstart)
	iw.writeFolded("SUMMARY:" + icalEscapeText(summary))
	if description != "" {
		iw.writeFolded("DESCRIPTION:" + icalEscapeText(description))
	}
	iw.writeLine("TRANSP:TRANSPARENT")
	iw.writeLine("END:VEVENT")
	return iw.err
}

func renderTemplate(tmpl *template.Template, data EventData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// icalEscapeText escapes a string for use as an iCalendar TEXT value as
// defined in RFC 5545 §3.3.11: backslash, semicolon, and comma are
// backslash-prefixed, and CR, LF, and CRLF are collapsed to the literal
// two-character escape sequence "\n".
func icalEscapeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case ';':
			b.WriteString(`\;`)
		case ',':
			b.WriteString(`\,`)
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			b.WriteString(`\n`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func generateUID(domain, calName, personName string, parts ...string) string {
	// NUL byte cannot appear in any of the inputs (parseLine rejects
	// control characters via TrimSpace and the line-length checks),
	// so it is an unambiguous field separator that prevents collisions
	// between e.g. (calName="a", personName="b/c") and
	// (calName="a/b", personName="c").
	var key strings.Builder
	key.WriteString(calName)
	key.WriteByte(0)
	key.WriteString(personName)
	for _, p := range parts {
		key.WriteByte(0)
		key.WriteString(p)
	}
	h := sha256.Sum256([]byte(key.String()))
	return fmt.Sprintf("%x@%s", h, domain)
}

type icalWriter struct {
	w   io.Writer
	err error
}

func (iw *icalWriter) writeLine(line string) {
	if iw.err != nil {
		return
	}
	_, iw.err = io.WriteString(iw.w, line+"\r\n")
}

func (iw *icalWriter) writeFolded(line string) {
	if iw.err != nil {
		return
	}
	const maxLen = 75
	for len(line) > maxLen {
		cut := maxLen
		for cut > 0 && line[cut]&0xC0 == 0x80 {
			cut--
		}
		_, iw.err = io.WriteString(iw.w, line[:cut]+"\r\n")
		if iw.err != nil {
			return
		}
		line = " " + line[cut:]
	}
	_, iw.err = io.WriteString(iw.w, line+"\r\n")
}
