package note

import (
	"testing"
	"time"
)

func TestParseDueInput_AbsoluteDate(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	got, err := ParseDueInput("2026-05-01", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDueInput_Days(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	got, err := ParseDueInput("3 days", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDueInput_DaySingular(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	got, err := ParseDueInput("1 day", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDueInput_Weeks(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	got, err := ParseDueInput("2 weeks", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDueInput_WeekSingular(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	got, err := ParseDueInput("1 week", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDueInput_Months(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	got, err := ParseDueInput("3 months", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDueInput_MonthSingular(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	got, err := ParseDueInput("1 month", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseDueInput_Invalid(t *testing.T) {
	now := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

	cases := []string{
		"not-a-date",
		"2 years",
		"abc days",
		"",
		"weeks 2",
	}
	for _, input := range cases {
		_, err := ParseDueInput(input, now)
		if err == nil {
			t.Errorf("expected error for input %q, got nil", input)
		}
	}
}
