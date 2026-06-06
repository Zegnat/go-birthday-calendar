// SPDX-FileCopyrightText: 2026 Martijn van der Ven
// SPDX-License-Identifier: 0BSD

package birthdaycal

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func parseLine(line string) (Birthday, error) {
	if len(line) < 11 || line[10] != ' ' {
		return Birthday{}, fmt.Errorf("expected 'YYYY-MM-DD Name', got %q", line)
	}

	datePart := line[:10]
	name := strings.TrimSpace(line[11:])

	if name == "" {
		return Birthday{}, fmt.Errorf("missing name after date")
	}

	b, err := parseDate(datePart)
	if err != nil {
		return Birthday{}, err
	}
	b.Name = name

	return b, nil
}

func parseDate(s string) (Birthday, error) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return Birthday{}, fmt.Errorf("invalid date format %q, expected YYYY-MM-DD", s)
	}

	yearStr := s[0:4]
	monthStr := s[5:7]
	dayStr := s[8:10]

	yearVal, err := strconv.Atoi(yearStr)
	if err != nil {
		return Birthday{}, fmt.Errorf("invalid year %q", yearStr)
	}

	monthVal, err := strconv.Atoi(monthStr)
	if err != nil {
		return Birthday{}, fmt.Errorf("invalid month %q", monthStr)
	}
	if monthVal < 1 || monthVal > 12 {
		return Birthday{}, fmt.Errorf("month %d out of range 1-12", monthVal)
	}

	dayVal, err := strconv.Atoi(dayStr)
	if err != nil {
		return Birthday{}, fmt.Errorf("invalid day %q", dayStr)
	}

	maxDay := daysInMonth(time.Month(monthVal), yearVal)
	if dayVal < 1 || dayVal > maxDay {
		return Birthday{}, fmt.Errorf("day %d out of range 1-%d for month %d", dayVal, maxDay, monthVal)
	}

	var year *int
	if yearVal != 0 {
		year = &yearVal
	}

	return Birthday{
		Month: time.Month(monthVal),
		Day:   dayVal,
		Year:  year,
	}, nil
}

func daysInMonth(m time.Month, year int) int {
	switch m {
	case time.February:
		if year == 0 || isLeapYear(year) {
			return 29
		}
		return 28
	case time.April, time.June, time.September, time.November:
		return 30
	default:
		return 31
	}
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
