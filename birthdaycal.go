// SPDX-FileCopyrightText: 2026 Martijn van der Ven
// SPDX-License-Identifier: 0BSD

// Package birthdaycal converts a plain-text list of birthdays into an
// iCalendar (RFC 5545) stream of recurring birthday events.
//
// The input format is one entry per line: an ISO date (YYYY-MM-DD)
// followed by a space and the person's name. The literal year 0000
// marks an unknown birth year. Blank lines and lines beginning with
// '#' are ignored.
//
// Call [Generate] to stream a calendar to any io.Writer. Customise the
// output via [Options] — for example, by configuring per-event title
// and description templates or by enabling a "unique-age" event window
// around a reference date.
package birthdaycal

import (
	"fmt"
	"text/template"
	"time"
)

// Birthday is a single entry parsed from the input file: the person's
// name and their month/day, with the year optionally present.
type Birthday struct {
	// Name of the person whose birthday this is.
	Name string
	// Month of the birthday (1-12).
	Month time.Month
	// Day of the birthday (1-31).
	Day int
	// Year of birth, or nil when the year is unknown.
	Year *int
}

// ParseError describes a problem encountered while parsing a single
// line of input. It implements the error interface and is the error
// type returned by [Generate] for input-format failures.
type ParseError struct {
	// File is the input file name, when known. Empty when the input
	// is unnamed (e.g. an io.Reader).
	File string
	// Line is the 1-based line number where the error occurred.
	Line int
	// Message describes what was wrong with the line.
	Message string
}

// Error renders the ParseError as "file:line: message" or
// "line N: message" when no file name is set.
func (e *ParseError) Error() string {
	if e.File == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Message)
}

// Date is a month-and-day pair with an optional year. It is exposed to
// the event title and description templates as EventData.BirthDate so
// templates can branch on whether a year is known and format the date.
type Date struct {
	// Month of the date (1-12).
	Month time.Month
	// Day of the date (1-31).
	Day int
	// Year of the date, or nil when the year is unknown.
	Year *int
}

// HasYear reports whether the Date carries a known year.
func (d Date) HasYear() bool { return d.Year != nil }

// Format renders the Date using a Go time-package reference layout
// (e.g. "2006-01-02"). When the year is unknown, year zero is used —
// templates should guard with HasYear before formatting a layout that
// includes the year.
func (d Date) Format(layout string) string {
	y := 0
	if d.Year != nil {
		y = *d.Year
	}
	return time.Date(y, d.Month, d.Day, 0, 0, 0, 0, time.UTC).Format(layout)
}

// EventData is the value passed to the title and description templates
// when rendering a single VEVENT.
type EventData struct {
	// Name of the person.
	Name string
	// BirthDate is the parsed birthday, including the year when known.
	BirthDate Date
	// Age is the person's age at the rendered event, formatted as a
	// decimal string. Empty for events outside the "unique" window
	// configured via Options.UniquePast / Options.UniqueFuture, and
	// for birthdays whose year is unknown.
	Age string
}

// DefaultTitleTemplate is the [text/template] source used for the
// VEVENT SUMMARY when [Options.Title] is nil. It renders as
// "Name (age)" when an age is known and "Name (birth-year)" otherwise.
const DefaultTitleTemplate = `{{.Name}}{{if .Age}} ({{.Age}}){{else if .BirthDate.HasYear}} ({{.BirthDate.Format "2006"}}){{end}}`

// DefaultDescriptionTemplate is the [text/template] source used for
// the VEVENT DESCRIPTION when [Options.Description] is nil. It renders
// as the ISO birth date when the year is known and is empty
// otherwise.
const DefaultDescriptionTemplate = `{{if .BirthDate.HasYear}}{{.BirthDate.Format "2006-01-02"}}{{end}}`

// Options configures a Generate invocation. The zero value is valid:
// Generate will fill in a default PRODID, default title and
// description templates, and use time.Now as the reference date.
type Options struct {
	// CalendarName is the iCalendar X-WR-CALNAME value.
	CalendarName string
	// Now is the reference date used for the unique-age event window
	// (UniquePast / UniqueFuture). When zero, time.Now is used.
	Now time.Time
	// UniquePast is the number of years before Now that receive an
	// individual age-stamped VEVENT in addition to the yearly RRULE.
	UniquePast int
	// UniqueFuture is the number of years after Now that receive an
	// individual age-stamped VEVENT in addition to the yearly RRULE.
	UniqueFuture int
	// PRODID is the iCalendar PRODID property value.
	PRODID string
	// UIDDomain is the right-hand side of each VEVENT UID, after the
	// "@". RFC 5545 §3.8.4.7 recommends a domain owned by the
	// calendar producer so UIDs are globally unique across feeds.
	// When empty, "birthday-calendar" is used.
	UIDDomain string
	// Title, when non-nil, overrides the default VEVENT SUMMARY
	// template. The template receives an EventData value.
	Title *template.Template
	// Description, when non-nil, overrides the default VEVENT
	// DESCRIPTION template. The template receives an EventData value.
	// An empty rendered string suppresses the DESCRIPTION property.
	Description *template.Template
}
