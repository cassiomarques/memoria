package note

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDueInput parses a due date string. It accepts:
//   - Absolute dates: "2026-04-20" (YYYY-MM-DD)
//   - Relative durations: "2 weeks", "3 days", "1 month"
//
// Returns the resolved time or an error.
// The now parameter allows testing with a fixed reference time.
func ParseDueInput(input string, now time.Time) (time.Time, error) {
	// Try absolute date first.
	if t, err := time.Parse(time.DateOnly, input); err == nil {
		return t, nil
	}

	// Try relative duration: "<number> <unit>"
	parts := strings.Fields(input)
	if len(parts) == 2 {
		n, err := strconv.Atoi(parts[0])
		if err == nil {
			unit := strings.TrimSuffix(strings.ToLower(parts[1]), "s") // normalize "weeks" → "week"
			switch unit {
			case "day":
				return now.AddDate(0, 0, n), nil
			case "week":
				return now.AddDate(0, 0, n*7), nil
			case "month":
				return now.AddDate(0, n, 0), nil
			}
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized due date %q (use YYYY-MM-DD or e.g. \"2 weeks\")", input)
}
