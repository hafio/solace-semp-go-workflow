// Package semp implements the low-level SEMP v2 Config API client and the
// path/payload helper utilities shared by the action modules.
package semp

import (
	"fmt"
	"strconv"
	"strings"
)

// TopicMaxLength is the broker's limit on a topic string, in bytes. It applies to
// every subscription, exception and topic-shaped identifier rather than to a
// single attribute, so the topic entries below all reference it.
const TopicMaxLength = 250

// NameMaxLengths holds the broker-enforced length limits for identifying
// attributes, checked before each request so an over-long value fails locally
// instead of costing a round trip.
//
// The limits are not in the SEMP schema: `maxLength` appears nowhere in the
// specification, and the v1 CLI schema's parameter types do not map reliably onto
// v2 attribute names. They are curated from Solace product documentation.
//
// Limits are in BYTES, matching both the broker and CheckNameLength's use of
// len(). Do not change that to a rune count: a multi-byte topic would then be
// accepted locally and rejected by the broker.
//
// An attribute absent from this map is simply not checked, so partial coverage is
// safe. Note the map is keyed by attribute name alone: if two objects ever share
// a name with different limits, this would need the object prefix as well.
var NameMaxLengths = map[string]int{
	// Message VPN and its access control.
	"msgVpnName":        32,
	"aclProfileName":    32,
	"clientProfileName": 32,
	"clientUsername":    189,

	// Endpoints.
	"queueName":                 200,
	"queueTemplateName":         255,
	"topicEndpointName":         250,
	"topicEndpointTemplateName": 255,
	"replayLogName":             185,

	// REST delivery.
	"restDeliveryPointName": 100,
	"restConsumerName":      32,
	"queueBindingName":      200,
	"headerName":            50,
	"oauthJwtClaimName":     100,

	// Bridges and DMR.
	"bridgeName":       150,
	"remoteMsgVpnName": 32,
	"dmrClusterName":   64,
	"remoteNodeName":   64,

	// Distributed cache.
	"cacheName":       200,
	"clusterName":     200,
	"homeClusterName": 200,
	"instanceName":    200,

	// Certificates and TLS.
	"certAuthorityName":        64,
	"ocspTlsTrustedCommonName": 64,
	"tlsTrustedCommonName":     64,
	"ruleName":                 64,
	"filterName":               64,

	// Authentication and authorization.
	"oauthProfileName":                32,
	"oauthProviderName":               32,
	"clientRequiredClaimName":         100,
	"resourceServerRequiredClaimName": 100,
	"authorizationParameterName":      32,
	"groupName":                       256,
	"authorizationGroupName":          256,
	// The broker also enforces a minimum of 2 for a Kerberos realm, which this
	// map cannot express; a one-character realm is rejected by the broker.
	"kerberosRealmName": 254,

	// Kafka bridging (software broker only).
	"kafkaSenderName":   100,
	"kafkaReceiverName": 100,

	// Telemetry.
	"telemetryProfileName": 21,
	"traceFilterName":      127,

	// Body attributes naming a Solace client username. The other *Username
	// attributes on these objects are credentials for a remote system, not
	// broker client usernames, so they are deliberately not limited here.
	"remoteAuthenticationBasicClientUsername":            189,
	"replicationBridgeAuthenticationBasicClientUsername": 189,
	// A queue, topic endpoint or MQTT session owner is a client username.
	"owner": 189,

	// Body attributes naming another object, which take that object's limit.
	// Several are documented 0..N -- empty is allowed, and CleanPayload drops a
	// blank value before it is sent, so only the maximum needs enforcing.
	"deadMsgQueue":                200, // queue, queue_template, topic_endpoint, topic_endpoint_template
	"queueDeadMsgQueue":           200, // dmr_cluster_link, mqtt_session
	"queueBinding":                200, // bridge_remote_msg_vpn
	"unidirectionalClientProfile": 32,  // bridge_remote_msg_vpn
	"replicationBridgeUnidirectionalClientProfileName":        32,  // msg_vpn
	"authenticationOauthDefaultProfileName":                   32,  // msg_vpn
	"oauthProfileDefault":                                     32,  // broker
	"authenticationOauthClientProxyName":                      32,  // rdp_rc
	"authenticationOauthJwtProxyName":                         32,  // rdp_rc
	"apiQueueManagementCopyFromOnCreateTemplateName":          255, // client_profile
	"apiTopicEndpointManagementCopyFromOnCreateTemplateName":  255, // client_profile

	// These three do NOT take the limit of the thing they appear to name, which is
	// why they are stated rather than inherited:
	//   localTopic is a substitution expression that generates a topic, not a
	//     topic, so it is not bound by the 250-byte topic limit.
	//   remoteTopic is a Kafka topic, and Kafka's limit is its own.
	//   physicalName is a JMS destination name, not a Solace queue name.
	"localTopic":   1024, // kafka_receiver_topic_binding
	"remoteTopic":  249,  // kafka_sender_queue_binding
	"physicalName": 256,  // jndi_queue, jndi_topic

	"alias": 32, // msg_vpn -- names another message VPN

	// Deliberately NOT limited, so this is not re-analysed:
	//
	//   - Attributes naming something in an external system: LDAP and RADIUS
	//     profiles, JWT claim names, Kerberos principals and keytabs, OAuth client
	//     ids, AWS identifiers, remote hostnames, and the usernames used to log in
	//     to a remote broker or REST host. Their limits belong to that system, not
	//     to Solace, and guessing one rejects input the broker would accept.
	//   - Secrets and certificate content: write-only, masked, and re-sent on every
	//     update regardless of length.
	//   - Lists and expressions: cipher-suite lists, bootstrap address lists,
	//     exclude lists, substitution expressions, regular-expression filters.
	//
	// An attribute with no entry here is simply not length-checked. The broker
	// still rejects an over-long value; the only cost is a round trip instead of a
	// local error.

	// JNDI, proxies, hostnames, attributes.
	"connectionFactoryName": 256,
	"topicName":             255,
	"proxyName":             32,
	"virtualHostname":       253,
	"attributeName":         64,

	// Topic-shaped identifiers. These are subscriptions, exceptions and topic
	// filters rather than object names, so they take the broker's topic limit.
	"subscriptionTopic":           TopicMaxLength,
	"remoteSubscriptionTopic":     TopicMaxLength,
	"topicFilterSubscription":     TopicMaxLength,
	"subscription":                TopicMaxLength,
	"publishTopicException":       TopicMaxLength,
	"subscribeTopicException":     TopicMaxLength,
	"publishExceptionTopic":       TopicMaxLength,
	"subscribeExceptionTopic":     TopicMaxLength,
	"subscribeShareNameException": TopicMaxLength,
	"replicatedTopic":             TopicMaxLength,
	"sequencedTopic":              TopicMaxLength,
	"topic":                       TopicMaxLength,
	"topicPrefix":                 TopicMaxLength,
}

// CheckNameLength returns an error message if value exceeds the broker limit for
// an attribute, or an empty string when it is within the limit (or the attribute
// has no known limit). An empty return means "no error".
//
// The limit is looked up by wireField, the SEMP spelling, because NameMaxLengths
// is keyed that way -- the limits come from Solace's documentation, which names
// SEMP attributes. The message quotes displayField, the spelling the caller
// actually typed, so an operator writing snake_case is not told about a name they
// never wrote. Pass the same string for both when they are identical.
func CheckNameLength(wireField, displayField, value string) string {
	limit, ok := NameMaxLengths[wireField]
	// len() counts bytes, which is what the broker limits. A rune count would
	// accept a multi-byte topic here and have it rejected on the wire.
	if ok && len(value) > limit {
		return fmt.Sprintf(
			"'%s' value is %d bytes but the broker limit is %d: '%s'",
			displayField, len(value), limit, value,
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
