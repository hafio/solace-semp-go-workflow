package semp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringify(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, "None"},
		{"true", true, "True"},
		{"false", false, "False"},
		{"string", "abc", "abc"},
		{"int", 5, "5"},
		{"int64", int64(7), "7"},
		{"float whole", 5.0, "5.0"},
		{"float frac", 5.5, "5.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Stringify(tc.in))
		})
	}
}

func TestEnc(t *testing.T) {
	// Every byte but the unreserved set is percent-encoded with uppercase hex;
	// '/' is NOT left alone.
	assert.Equal(t, "a%20b", Enc("a b"))
	assert.Equal(t, "SITEA%2FAPP%2F%3E", Enc("SITEA/APP/>"))
	assert.Equal(t, "abc-_.~", Enc("abc-_.~"))
	assert.Equal(t, "a%40b", Enc("a@b"))
}

func TestCoerceBool(t *testing.T) {
	truthy := []any{true, "true", "TRUE", "Yes", "1", 1, int64(2), 3.5}
	for _, v := range truthy {
		assert.Truef(t, CoerceBool(v), "expected %v truthy", v)
	}
	falsy := []any{false, "false", "no", "", "0", nil, 0, int64(0), 0.0}
	for _, v := range falsy {
		assert.Falsef(t, CoerceBool(v), "expected %v falsy", v)
	}
}

func TestCoerceInt(t *testing.T) {
	n, err := CoerceInt(5)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = CoerceInt(int64(9))
	require.NoError(t, err)
	assert.Equal(t, 9, n)

	n, err = CoerceInt("10")
	require.NoError(t, err)
	assert.Equal(t, 10, n)

	n, err = CoerceInt("  42 ")
	require.NoError(t, err)
	assert.Equal(t, 42, n)

	// A float stringifies to "5.0", which is not a valid base-10 int, so this errors.
	_, err = CoerceInt(5.0)
	assert.Error(t, err)

	_, err = CoerceInt("abc")
	assert.Error(t, err)
}

func TestCleanPayload(t *testing.T) {
	in := map[string]any{
		"nilVal":   nil,
		"blank":    "   ",
		"empty":    "",
		"keep":     "x",
		"zero":     0,
		"falseVal": false,
	}
	out := CleanPayload(in)
	assert.NotContains(t, out, "nilVal")
	assert.NotContains(t, out, "blank")
	assert.NotContains(t, out, "empty")
	assert.Equal(t, "x", out["keep"])
	assert.Equal(t, 0, out["zero"])
	assert.Equal(t, false, out["falseVal"])
}

// checkLen looks up and reports under the same name, which is the case for an
// attribute spelled identically in both conventions. The two names diverging is
// exercised in TestCheckNameLengthReportsTheNameTheCallerTyped.
func checkLen(field, value string) string { return CheckNameLength(field, field, value) }

// The limits are keyed by the SEMP spelling because they come from Solace's
// documentation, but an operator writing the canonical snake_case name must not
// be told about a name they never typed.
func TestCheckNameLengthReportsTheNameTheCallerTyped(t *testing.T) {
	msg := CheckNameLength("queueName", "queue_name", strings.Repeat("x", 201))

	require.NotEmpty(t, msg, "the limit must still be found under the SEMP spelling")
	assert.Contains(t, msg, "queue_name")
	assert.NotContains(t, msg, "queueName")
	assert.Contains(t, msg, "200")
}

func TestCheckNameLength(t *testing.T) {
	assert.Equal(t, "", checkLen("queueName", "short"))
	assert.Equal(t, "", checkLen("unknownField", strings.Repeat("x", 500)))

	msg := checkLen("queueName", strings.Repeat("x", 201))
	assert.Contains(t, msg, "200")
	assert.Contains(t, msg, "queueName")

	// Boundary: exactly the limit is allowed.
	assert.Equal(t, "", checkLen("restConsumerName", strings.Repeat("x", 32)))
	assert.NotEqual(t, "", checkLen("restConsumerName", strings.Repeat("x", 33)))
}

func TestCheckNameLengthCountsBytesNotRunes(t *testing.T) {
	// The broker limits bytes. A rune count would accept this locally and have the
	// broker reject it, which is the failure mode the local check exists to avoid.
	// "e" with a combining acute is 2 bytes per character.
	value := strings.Repeat("é", 20) // 20 runes, 40 bytes
	assert.Equal(t, 40, len(value))

	// telemetryProfileName has the tightest limit in the map, at 21.
	msg := checkLen("telemetryProfileName", value)
	assert.NotEqual(t, "", msg, "40 bytes must exceed a 21-byte limit even though it is 20 characters")
	assert.Contains(t, msg, "40 bytes")
	assert.Contains(t, msg, "limit is 21")
}

func TestTopicIdentifiersUseTheTopicLimit(t *testing.T) {
	// Subscriptions, exceptions and topic filters are topic strings rather than
	// object names, so they share the broker's topic limit.
	for _, field := range []string{
		"subscriptionTopic", "remoteSubscriptionTopic", "topicFilterSubscription",
		"subscription", "publishTopicException", "subscribeTopicException",
		"publishExceptionTopic", "subscribeExceptionTopic", "subscribeShareNameException",
		"replicatedTopic", "sequencedTopic", "topic", "topicPrefix",
	} {
		t.Run(field, func(t *testing.T) {
			assert.Equal(t, TopicMaxLength, NameMaxLengths[field])
			assert.Equal(t, "", checkLen(field, strings.Repeat("a", TopicMaxLength)))
			assert.NotEqual(t, "", checkLen(field, strings.Repeat("a", TopicMaxLength+1)))
		})
	}
}

// The limits are curated from product documentation rather than derived, so this
// pins the set: a limit cannot be dropped or mistyped without failing here.
// Coverage of the descriptors' identifying keys is asserted in the modules
// package, which is where the descriptors live.
func TestCuratedLimitsAreSane(t *testing.T) {
	assert.Len(t, NameMaxLengths, 75, "44 identifying names, 13 topic-shaped, 18 body attributes")
	for field, limit := range NameMaxLengths {
		assert.Positivef(t, limit, "%s has a non-positive limit", field)
		assert.LessOrEqualf(t, limit, 1024, "%s exceeds any documented SEMP limit", field)
	}
}

// Three attributes look like they should inherit the limit of the thing they
// name and do not. Each was verified against the product documentation, and each
// inferred value was wrong, so they are pinned here against a future "tidy-up".
func TestAttributesThatDoNotInheritTheObviousLimit(t *testing.T) {
	// A substitution expression that generates a topic, not a topic.
	assert.Equal(t, 1024, NameMaxLengths["localTopic"])
	assert.NotEqual(t, TopicMaxLength, NameMaxLengths["localTopic"])

	// A Kafka topic, bound by Kafka's limit rather than Solace's.
	assert.Equal(t, 249, NameMaxLengths["remoteTopic"])
	assert.NotEqual(t, TopicMaxLength, NameMaxLengths["remoteTopic"])

	// A JMS destination name, not a Solace queue name.
	assert.Equal(t, 256, NameMaxLengths["physicalName"])
	assert.NotEqual(t, NameMaxLengths["queueName"], NameMaxLengths["physicalName"])
}

// owner is a client username, which a name-suffix reading of the attribute list
// misses entirely. It appears on three object types.
func TestOwnerTakesTheClientUsernameLimit(t *testing.T) {
	assert.Equal(t, NameMaxLengths["clientUsername"], NameMaxLengths["owner"])
	assert.Equal(t, "", checkLen("owner", strings.Repeat("u", 189)))
	assert.NotEqual(t, "", checkLen("owner", strings.Repeat("u", 190)))
}
