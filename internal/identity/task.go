package identity

import (
	"fmt"
	"regexp"
	"strings"
)

var taskIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func ValidateTaskID(value string) error {
	if !taskIDPattern.MatchString(value) {
		return fmt.Errorf("task ID must match %s", taskIDPattern.String())
	}
	return nil
}

func NormalizeTaskID(value string) string {
	var out strings.Builder
	lastSeparator := false
	for _, r := range value {
		allowed := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if allowed {
			out.WriteRune(r)
			lastSeparator = false
		} else if !lastSeparator {
			out.WriteByte('-')
			lastSeparator = true
		}
		if out.Len() >= 64 {
			break
		}
	}
	result := strings.Trim(out.String(), "._-")
	if result == "" {
		return "stateseal-task"
	}
	return result
}
