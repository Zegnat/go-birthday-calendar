<!--
SPDX-FileCopyrightText: 2026 Martijn van der Ven
SPDX-License-Identifier: 0BSD
-->

# go-birthday-calendar

[![Check](https://github.com/Zegnat/go-birthday-calendar/actions/workflows/check.yml/badge.svg)](https://github.com/Zegnat/go-birthday-calendar/actions/workflows/check.yml)
[![Release](https://img.shields.io/github/v/release/Zegnat/go-birthday-calendar?display_name=tag&sort=semver)](https://github.com/Zegnat/go-birthday-calendar/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/Zegnat/go-birthday-calendar.svg)](https://pkg.go.dev/github.com/Zegnat/go-birthday-calendar)
[![Go Report Card](https://goreportcard.com/badge/github.com/Zegnat/go-birthday-calendar)](https://goreportcard.com/report/github.com/Zegnat/go-birthday-calendar)
[![Coverage](https://raw.githubusercontent.com/Zegnat/go-birthday-calendar/badges/.badges/main/coverage.svg)](https://github.com/Zegnat/go-birthday-calendar/actions/workflows/check.yml)
[![REUSE status](https://api.reuse.software/badge/github.com/Zegnat/go-birthday-calendar)](https://api.reuse.software/info/github.com/Zegnat/go-birthday-calendar)

Turn a plain-text list of birthdays into a recurring
[iCalendar](https://datatracker.ietf.org/doc/html/rfc5545) (`.ics`)
file. Keep your data in a text file, regenerate the `.ics` on a cron,
host it as a URL — any calendar app can subscribe.

## How

1. Write a text file of birthdays.
2. Run `birthday-calendar -i family.txt -o family.ics`.
3. Host `family.ics` on any static web host.
4. **Subscribe to calendar by URL** in your calendar app.

## Install

Pre-built binaries for Linux and macOS (amd64 + arm64) are attached to
every [release](https://github.com/Zegnat/go-birthday-calendar/releases).
`checksums.txt` is signed with cosign keyless against the GitHub
Actions release workflow.

If you prefer to build from source (Go 1.24+):

```sh
go install github.com/Zegnat/go-birthday-calendar/cmd/birthday-calendar@latest
```

## Input

One person per line: ISO date and name. `0000` for unknown year.
Lines starting with `#` and blank lines are ignored.

```text
1990-03-15 Alice
0000-07-22 Bob
1985-12-01 Carol
```

## Usage

```sh
birthday-calendar -i family.txt -o family.ics
```

Reads stdin and writes stdout by default.

| Flag                        | Default      | Description                                          |
| --------------------------- | ------------ | ---------------------------------------------------- |
| `-i`, `--input`             | `-`          | Input file (`-` = stdin).                            |
| `-o`, `--output`            | `-`          | Output file (`-` = stdout).                          |
| `-n`, `--name`              | filename     | Calendar name (`X-WR-CALNAME`).                      |
| `-p`, `--unique-past`       | `0`          | Years before `--now` that get age-stamped events.    |
| `-f`, `--unique-future`     | `0`          | Years after `--now` that get age-stamped events.     |
| `-t`, `--event-title`       | see `--help` | Go template for each event's `SUMMARY`.              |
| `-d`, `--event-description` | see `--help` | Go template for each event's `DESCRIPTION`.          |
| `--prodid`                  | derived      | iCalendar `PRODID` identifier.                       |
| `--uid-domain`              | derived      | Right-hand side of each `UID` after `@`.             |
| `--now`                     | system clock | Reference date (`YYYY-MM-DD`) for the unique window. |
| `-v`, `--version`           |              | Print the version and exit.                          |

`--unique-past N --unique-future M` makes events around `--now`
include the person's age (`Alice (36)`); outside that window the
title falls back to the birth year (`Alice (1990)`). `-p 3 -f 5` is a
reasonable default for "show the next few years' ages."

The default `--event-title` and `--event-description` templates
receive an
[`EventData`](https://pkg.go.dev/github.com/Zegnat/go-birthday-calendar#EventData)
value and use Go's
[`text/template`](https://pkg.go.dev/text/template) syntax. Run
`birthday-calendar --help` for the literal default strings.

## Library

```go
import birthdaycal "github.com/Zegnat/go-birthday-calendar"

err := birthdaycal.Generate(in, out, birthdaycal.Options{CalendarName: "family"})
```

Full API on [pkg.go.dev](https://pkg.go.dev/github.com/Zegnat/go-birthday-calendar).
