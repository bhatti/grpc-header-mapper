package headermapper

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Advanced transformation functions

// Normalize normalizes header values by trimming space and converting to lowercase
func Normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// SanitizeUserAgent sanitizes user agent strings by removing sensitive information
func SanitizeUserAgent(value string) string {
	// Remove version numbers and specific system information
	re := regexp.MustCompile(`\d+\.\d+(\.\d+)*`)
	return re.ReplaceAllString(value, "x.x.x")
}

// FormatTimestamp formats Unix timestamp to ISO 8601
func FormatTimestamp(value string) string {
	if timestamp, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(timestamp, 0).UTC().Format(time.RFC3339)
	}
	return value
}

// ParseTimestamp parses ISO 8601 to Unix timestamp
func ParseTimestamp(value string) string {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return strconv.FormatInt(t.Unix(), 10)
	}
	return value
}

// ExtractBearerToken extracts the token from "Bearer <token>" format
func ExtractBearerToken(value string) string {
	const bearerPrefix = "Bearer "
	if strings.HasPrefix(value, bearerPrefix) {
		return strings.TrimSpace(value[len(bearerPrefix):])
	}
	return value
}

// MaskSensitive masks sensitive information, showing only first and last few characters
func MaskSensitive(showChars int) TransformFunc {
	return func(value string) string {
		if len(value) <= showChars*2 {
			return strings.Repeat("*", len(value))
		}
		return value[:showChars] + strings.Repeat("*", len(value)-showChars*2) + value[len(value)-showChars:]
	}
}

// ConditionalTransform applies a transform only if condition is met
func ConditionalTransform(condition func(string) bool, transform TransformFunc) TransformFunc {
	return func(value string) string {
		if condition(value) {
			return transform(value)
		}
		return value
	}
}

// RegexReplace performs regex-based replacement
func RegexReplace(pattern, replacement string) TransformFunc {
	re := regexp.MustCompile(pattern)
	return func(value string) string {
		return re.ReplaceAllString(value, replacement)
	}
}

// Truncate truncates the value to a maximum length
func Truncate(maxLength int) TransformFunc {
	return func(value string) string {
		if len(value) <= maxLength {
			return value
		}
		return value[:maxLength]
	}
}

// AddSuffix adds a suffix to the value
func AddSuffix(suffix string) TransformFunc {
	return func(value string) string {
		return value + suffix
	}
}

// RemoveSuffix removes a suffix from the value
func RemoveSuffix(suffix string) TransformFunc {
	return func(value string) string {
		return strings.TrimSuffix(value, suffix)
	}
}

// DefaultIfEmpty returns a default value if the input is empty
func DefaultIfEmpty(defaultValue string) TransformFunc {
	return func(value string) string {
		if strings.TrimSpace(value) == "" {
			return defaultValue
		}
		return value
	}
}
