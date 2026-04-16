package domain

import "regexp"

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// IsUUID returns true if s matches UUID format (lowercase hex with dashes).
func IsUUID(s string) bool {
	return uuidPattern.MatchString(s)
}
