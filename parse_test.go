// SPDX-FileCopyrightText: 2026 Martijn van der Ven
// SPDX-License-Identifier: 0BSD

package birthdaycal

import (
	"testing"
	"time"
)

func TestParseErrorError(t *testing.T) {
	tests := []struct {
		e    *ParseError
		want string
	}{
		{&ParseError{Line: 5, Message: "bad"}, "line 5: bad"},
		{&ParseError{File: "foo.txt", Line: 12, Message: "boom"}, "foo.txt:12: boom"},
	}
	for _, tc := range tests {
		if got := tc.e.Error(); got != tc.want {
			t.Errorf("ParseError.Error() = %q, want %q", got, tc.want)
		}
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantErr   bool
		wantName  string
		wantMonth time.Month
		wantDay   int
		// wantYear: -1 = no expectation, 0 = expect nil pointer.
		wantYear int
	}{
		{
			name:      "valid full date",
			in:        "1990-03-15 Alice van der Ven",
			wantName:  "Alice van der Ven",
			wantMonth: time.March,
			wantDay:   15,
			wantYear:  1990,
		},
		{
			name:      "unknown year",
			in:        "0000-07-22 Bob Example",
			wantName:  "Bob Example",
			wantMonth: time.July,
			wantDay:   22,
			wantYear:  0,
		},
		{
			name:      "feb 29 leap400",
			in:        "2000-02-29 Leap400 X",
			wantMonth: time.February,
			wantDay:   29,
			wantYear:  2000,
		},
		{
			name:      "feb 29 leap4",
			in:        "1992-02-29 Leap4 X",
			wantMonth: time.February,
			wantDay:   29,
			wantYear:  1992,
		},
		{
			name:      "feb 29 unknown year",
			in:        "0000-02-29 Unknown X",
			wantMonth: time.February,
			wantDay:   29,
			wantYear:  0,
		},
		{name: "feb 29 non-leap", in: "1991-02-29 NotLeap X", wantErr: true},
		{name: "feb 29 non-leap century", in: "1900-02-29 X", wantErr: true},
		{name: "feb 29 non-leap century 2100", in: "2100-02-29 X", wantErr: true},
		{name: "feb 30", in: "1990-02-30 Someone", wantErr: true},
		{name: "month 13", in: "1990-13-01 Bad Month", wantErr: true},
		{name: "missing name", in: "1990-03-15 ", wantErr: true},
		{name: "too short", in: "short", wantErr: true},
		{name: "non-date prefix", in: "not-a-date Oops", wantErr: true},
		{name: "year not numeric", in: "abcd-01-01 X", wantErr: true},
		{name: "month not numeric", in: "1990-aa-01 X", wantErr: true},
		{name: "day not numeric", in: "1990-01-aa X", wantErr: true},
		{name: "30-day month upper bound", in: "1990-04-30 X", wantMonth: time.April, wantDay: 30, wantYear: 1990},
		{name: "april day 31 invalid", in: "1990-04-31 X", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := parseLine(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseLine(%q) succeeded; want error", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLine(%q) failed: %v", tc.in, err)
			}
			if tc.wantName != "" && b.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", b.Name, tc.wantName)
			}
			if b.Month != tc.wantMonth {
				t.Errorf("Month = %v, want %v", b.Month, tc.wantMonth)
			}
			if b.Day != tc.wantDay {
				t.Errorf("Day = %d, want %d", b.Day, tc.wantDay)
			}
			switch tc.wantYear {
			case 0:
				if b.Year != nil {
					t.Errorf("Year = %d, want nil", *b.Year)
				}
			default:
				if b.Year == nil {
					t.Errorf("Year = nil, want %d", tc.wantYear)
				} else if *b.Year != tc.wantYear {
					t.Errorf("Year = %d, want %d", *b.Year, tc.wantYear)
				}
			}
		})
	}
}
