package dateparse

import (
	"testing"
	"time"
)

func fixedNow() time.Time {
	return time.Date(2026, 8, 2, 12, 0, 0, 0, time.Local)
}

func TestParseAbsolute(t *testing.T) {
	now := fixedNow()
	for _, in := range []string{"2020-01-01", "2026-08-02", "2026-08-02 15:30", "2026/08/02"} {
		if _, err := Parse(in, now); err != nil {
			t.Errorf("Parse(%q) err = %v", in, err)
		}
	}
}

func TestParseRelative(t *testing.T) {
	now := fixedNow()

	// today → start of today
	got, err := Parse("today", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, now) {
		t.Errorf("today = %v, want %v", got, now)
	}

	// tomorrow → start of next day
	got, err = Parse("tomorrow", now)
	if err != nil {
		t.Fatal(err)
	}
	tomorrow := now.AddDate(0, 0, 1)
	if !sameDay(got, tomorrow) {
		t.Errorf("tomorrow = %v, want %v", got, tomorrow)
	}

	// next monday → the upcoming monday (>= now day); +7 if today is monday
	nextMon := nextWeekday(now, time.Monday)
	got, err = Parse("next monday", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, nextMon) {
		t.Errorf("next monday = %v, want %v", got, nextMon)
	}

	// in 3 days
	got, err = Parse("in 3 days", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, now.AddDate(0, 0, 3)) {
		t.Errorf("in 3 days = %v", got)
	}

	// 3d shortcut → 3 days
	got, err = Parse("3d", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, now.AddDate(0, 0, 3)) {
		t.Errorf("3d = %v", got)
	}

	// 2w shortcut → 14 days
	got, err = Parse("2w", now)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDay(got, now.AddDate(0, 0, 14)) {
		t.Errorf("2w = %v", got)
	}

	// 4h shortcut → now + 4 hours
	got, err = Parse("4h", now)
	if err != nil {
		t.Fatal(err)
	}
	want := now.Add(4 * time.Hour)
	if !got.Equal(want) {
		t.Errorf("4h = %v, want %v", got, want)
	}
}

func TestParseEnglishMonth(t *testing.T) {
	now := fixedNow()
	// "july 1 2034" → 2034-07-01
	got, err := Parse("july 1 2034", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2034, 7, 1, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("july 1 2034 = %v, want %v", got, want)
	}

	// "jan 1" → this year's january 1
	got, err = Parse("jan 1", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Year() != now.Year() || got.Month() != time.January || got.Day() != 1 {
		t.Errorf("jan 1 = %v", got)
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"?????", "not a date at all", ""} {
		if _, err := Parse(in, fixedNow()); err == nil {
			t.Errorf("Parse(%q): expected error, got nil", in)
		}
	}
}

func TestParseWhitespaceAndCase(t *testing.T) {
	now := fixedNow()
	if _, err := Parse("  Tomorrow  ", now); err != nil {
		t.Errorf("whitespace/case tolerance failed: %v", err)
	}
	if _, err := Parse("NEXT FRIDAY", now); err != nil {
		t.Errorf("upper-case weekday failed: %v", err)
	}
}

func TestParseEdgeCases(t *testing.T) {
	now := fixedNow() // 2026-08-02 12:00

	// Invalid calendar dates are rejected (time.ParseInLocation refuses 2/30).
	if _, err := Parse("2020-02-30", now); err == nil {
		t.Error("2020-02-30 should be rejected")
	}
	if _, err := Parse("2020-13-01", now); err == nil {
		t.Error("2020-13-01 should be rejected")
	}

	// Leap day is valid in a leap year.
	if _, err := Parse("2024-02-29", now); err != nil {
		t.Errorf("2024-02-29 should parse: %v", err)
	}

	// next with no weekday is rejected.
	if _, err := Parse("next", now); err == nil {
		t.Error(`"next" alone should be rejected`)
	}

	// zero durations are rejected (must be > 0).
	if _, err := Parse("0d", now); err == nil {
		t.Error(`"0d" should be rejected`)
	}
	if _, err := Parse("in 0 days", now); err == nil {
		t.Error(`"in 0 days" should be rejected`)
	}

	// bare weekday without "next" is intentionally unsupported.
	if _, err := Parse("monday", now); err == nil {
		t.Error(`bare "monday" should be rejected (requires "next")`)
	}

	// hour shorthand preserves the time of day.
	got, err := Parse("in 1 hour", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(time.Hour); !got.Equal(want) {
		t.Errorf(`"in 1 hour" = %v, want %v`, got, want)
	}
}
