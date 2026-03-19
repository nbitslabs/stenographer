package query

import (
	"fmt"
	"strconv"
	"time"
)

// ParseSince parses a --since value which can be:
//   - A duration: "15m", "1h", "24h", "7d"
//   - An ISO 8601 timestamp: "2024-01-15T10:00:00"
func ParseSince(s string) (int64, error) {
	d, err := parseDuration(s)
	if err == nil {
		return time.Now().Add(-d).Unix(), nil
	}
	return ParseTimestamp(s)
}

// ParseTimestamp parses a timestamp as ISO 8601 or Unix epoch seconds.
func ParseTimestamp(s string) (int64, error) {
	if epoch, err := strconv.ParseInt(s, 10, 64); err == nil {
		return epoch, nil
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.Unix(), nil
		}
	}

	return 0, fmt.Errorf("cannot parse timestamp: %s (expected ISO 8601 or Unix epoch)", s)
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", s)
	}

	switch unit {
	case 's':
		return time.Duration(n * float64(time.Second)), nil
	case 'm':
		return time.Duration(n * float64(time.Minute)), nil
	case 'h':
		return time.Duration(n * float64(time.Hour)), nil
	case 'd':
		return time.Duration(n * 24 * float64(time.Hour)), nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %c (expected s, m, h, d)", unit)
	}
}
