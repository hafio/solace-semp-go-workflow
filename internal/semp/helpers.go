// Package semp implements the low-level SEMP v2 Config API client and the
// path/payload helper utilities shared by the action modules.
package semp

import (
	"fmt"
	"strconv"
	"strings"
)

// NameMaxLengths holds the broker-enforced name length limits (not in the SEMP
// swagger schema -- runtime only), checked by this client before each request.
var NameMaxLengths = map[string]int{
	"queueName":             200,
	"restDeliveryPointName": 100,
	"restConsumerName":      32,
	"queueBindingName":      200,
	"aclProfileName":        32,
	"clientProfileName":     32,
	"clientUsername":        189,
}

// CheckNameLength returns an error message if value exceeds the broker limit for
// field, or an empty string when it is within the limit (or the field has no
// known limit). An empty return means "no error".
func CheckNameLength(field, value string) string {
	limit, ok := NameMaxLengths[field]
	if ok && len(value) > limit {
		return fmt.Sprintf(
			"'%s' value is %d characters but the broker limit is %d: '%s'",
			field, len(value), limit, value,
		)
	}
	return ""
}

// Enc URL-encodes a SEMP path segment: every byte except the unreserved set
// (A-Z a-z 0-9 - _ . ~) is percent-encoded with uppercase hex. Neither
// url.PathEscape (leaves '/') nor url.QueryEscape (space -> '+') matches.
func Enc(value string) string {
	var b strings.Builder
	for _, c := range []byte(value) {
		if isUnreserved(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z':
		return true
	case c >= 'a' && c <= 'z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '-' || c == '_' || c == '.' || c == '~':
		return true
	}
	return false
}

// CoerceBool coerces a value to bool, handling YAML bools and Jinja2 string
// output. Strings are true when lower-cased to one of "true", "yes", "1";
// everything else falls back to general truthiness (nil and zero are false).
func CoerceBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(v) {
		case "true", "yes", "1":
			return true
		}
		return false
	case nil:
		return false
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		// Any other non-nil value is truthy.
		return true
	}
}

// CoerceInt coerces a value to int. Native ints pass through (Go bools are not
// ints, so no bool guard is needed); everything else is parsed from its string
// form via Stringify, so a non-numeric string is rejected.
func CoerceInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	}
	s := strings.TrimSpace(Stringify(value))
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid literal for int(): '%s'", s)
	}
	return n, nil
}

// CleanPayload returns a copy of args with nil values and blank/whitespace-only
// string values removed. Zero, false, and empty collections are kept.
func CleanPayload(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for k, v := range args {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// Stringify renders a value to the canonical string form used by input coercion
// to the "string" type: bool -> "True"/"False", nil -> "None", int and int64 as
// decimal, and float64 always with a decimal point (5.0 -> "5.0").
func Stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		// Floats always show a decimal point (e.g. 5.0 -> "5.0").
		s := strconv.FormatFloat(v, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eEnN") { // n/N guards inf/nan spellings
			s += ".0"
		}
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}
