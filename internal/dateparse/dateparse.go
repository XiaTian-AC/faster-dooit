// Package dateparse parses a small, explicit subset of natural-language
// date inputs into a time.Time.
//
// Supported grammar (in order tried):
//   - "today" / "tomorrow"                (today → 00:00 of now's day;
//     tomorrow → 00:00 of next day)
//   - shorthand "3d" | "2w" | "1h"         (days / weeks / hours from now)
//   - "next <weekday>"                    (e.g. "next monday"; if today
//     is that weekday, +7 days)
//   - "in <n> <unit>s?"                   (e.g. "in 3 days", "in 2 weeks",
//     "in 1 hour")
//   - absolute YYYY-MM-DD [HH:MM]
//   - absolute YYYY/MM/DD [HH:MM]
//   - English month format "july 1 2034" / "jan 1"
//     (year defaults to current)
//
// Not a substitute for python-dateutil; dooit relies on dateutil for fuzzy
// parsing, but this port keeps only what we use in practice. Document the
// supported grammar in the README.
package dateparse

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Parse turns input into a time.Time relative to now.
// Returns an error that includes the original input on failure.
func Parse(input string, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(strings.ToLower(input))
	if s == "" {
		return time.Time{}, errors.New("dateparse: empty input")
	}

	// today / tomorrow — both snap to start-of-day
	if s == "today" {
		return startOfDay(now), nil
	}
	if s == "tomorrow" {
		return startOfDay(now.AddDate(0, 0, 1)), nil
	}

	// Shorthand Nx where x ∈ {d,w,h}
	if t, ok := parseShorthand(s, now); ok {
		return t, nil
	}

	// next <weekday>
	if rest, ok := strings.CutPrefix(s, "next "); ok {
		if t, ok := parseWeekday(rest, now); ok {
			return t, nil
		}
		return time.Time{}, fmt.Errorf("dateparse: unrecognised weekday in %q", input)
	}

	// in <n> <unit>s?
	if rest, ok := strings.CutPrefix(s, "in "); ok {
		if t, ok := parseInPhrase(rest, now); ok {
			return t, nil
		}
		return time.Time{}, fmt.Errorf("dateparse: cannot parse %q as relative duration", input)
	}

	// Absolute formats
	for _, layout := range absoluteLayouts {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t, nil
		}
	}

	// English month format "july 1 2034" / "jan 1"
	if t, ok := parseEnglishMonth(s, now); ok {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("dateparse: unrecognised format %q (supported: today, tomorrow, 3d/2w/1h, next <weekday>, in <n> <unit>, YYYY-MM-DD[ HH:MM], YYYY/MM/DD, \"july 1 2034\", \"jan 1\")", input)
}

// Normalize returns a display-friendly representation (currently just RFC3339
// local) for the parsed time. Kept for parity with the dooit plan's editor
// echo use-case.
func Normalize(input string, now time.Time) (string, error) {
	t, err := Parse(input, now)
	if err != nil {
		return "", err
	}
	return t.Format("2006-01-02 15:04"), nil
}

// absoluteLayouts tries these layouts in order. HH:MM is optional in the
// first two layouts; the trailing two are date-only.
var absoluteLayouts = []string{
	"2006-01-02 15:04",
	"2006/01/02 15:04",
	"2006-01-02",
	"2006/01/02",
}

func parseShorthand(s string, now time.Time) (time.Time, bool) {
	if len(s) < 2 {
		return time.Time{}, false
	}
	last := s[len(s)-1]
	n, err := atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	switch last {
	case 'd':
		return now.AddDate(0, 0, n), true
	case 'w':
		return now.AddDate(0, 0, n*7), true
	case 'h':
		return now.Add(time.Duration(n) * time.Hour), true
	}
	return time.Time{}, false
}

func parseWeekday(s string, now time.Time) (time.Time, bool) {
	wd, ok := parseWeekdayName(s)
	if !ok {
		return time.Time{}, false
	}
	return nextWeekday(now, wd), true
}

func parseWeekdayName(s string) (time.Weekday, bool) {
	switch s {
	case "sunday":
		return time.Sunday, true
	case "monday":
		return time.Monday, true
	case "tuesday":
		return time.Tuesday, true
	case "wednesday":
		return time.Wednesday, true
	case "thursday":
		return time.Thursday, true
	case "friday":
		return time.Friday, true
	case "saturday":
		return time.Saturday, true
	}
	return 0, false
}

// nextWeekday returns the next occurrence of target strictly after today.
// If today is already target, returns +7 days.
func nextWeekday(now time.Time, target time.Weekday) time.Time {
	diff := int(target - now.Weekday())
	if diff <= 0 {
		diff += 7
	}
	return startOfDay(now.AddDate(0, 0, diff))
}

func parseInPhrase(s string, now time.Time) (time.Time, bool) {
	// "3 days" | "2 weeks" | "1 hour" (and singular)
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return time.Time{}, false
	}
	n, err := atoi(parts[0])
	if err != nil || n <= 0 {
		return time.Time{}, false
	}
	switch parts[1] {
	case "day", "days":
		return now.AddDate(0, 0, n), true
	case "week", "weeks":
		return now.AddDate(0, 0, n*7), true
	case "hour", "hours":
		return now.Add(time.Duration(n) * time.Hour), true
	}
	return time.Time{}, false
}

func parseEnglishMonth(s string, now time.Time) (time.Time, bool) {
	// "july 1 2034" / "july 1" / "jan 1"
	parts := strings.Fields(s)
	if len(parts) != 3 && len(parts) != 2 {
		return time.Time{}, false
	}
	month, ok := parseMonthName(parts[0])
	if !ok {
		return time.Time{}, false
	}
	day, err := atoi(parts[1])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	year := now.Year()
	if len(parts) == 3 {
		y, err := atoi(parts[2])
		if err != nil || y < 1900 || y > 9999 {
			return time.Time{}, false
		}
		year = y
	}
	return time.Date(year, month, day, 0, 0, 0, 0, now.Location()), true
}

func parseMonthName(s string) (time.Month, bool) {
	switch s {
	case "january", "jan":
		return time.January, true
	case "february", "feb":
		return time.February, true
	case "march", "mar":
		return time.March, true
	case "april", "apr":
		return time.April, true
	case "may":
		return time.May, true
	case "june", "jun":
		return time.June, true
	case "july", "jul":
		return time.July, true
	case "august", "aug":
		return time.August, true
	case "september", "sep", "sept":
		return time.September, true
	case "october", "oct":
		return time.October, true
	case "november", "nov":
		return time.November, true
	case "december", "dec":
		return time.December, true
	}
	return 0, false
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// atoi is a tiny int parser; avoids pulling strconv just for non-negative
// numeric runs.
func atoi(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
