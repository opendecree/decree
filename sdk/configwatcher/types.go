package configwatcher

import (
	"strconv"
	"time"
)

func parseString(s string) (string, error) {
	return s, nil
}

func parseInt(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func parseBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
