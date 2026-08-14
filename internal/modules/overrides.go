package modules

// This file is hand-written and never generated. It is the seam for per-object
// behaviour that a descriptor cannot express as data: derived payload fields,
// sentinel values, and the one object whose add action deliberately skips its
// pre-check. registerObject applies the entry for a prefix before constructing
// the drivers, so a generated descriptor never has to know that its object has
// bespoke behaviour and file init() order is irrelevant.
//
// Message wording is NOT overridden here -- it is data, and lives in the
// descriptor's own Msg field.
//
// Keep this map small. If an entry here grows beyond a few lines, that is a
// signal the driver is missing a descriptor field the specification could
// supply for every object rather than just this one.
var overrides = map[string]func(*objectDesc){
	"msg_vpn_queue": func(d *objectDesc) {
		d.AddHook = queueDerivedFields
		d.UpdateHook = queueDerivedFields
	},

	// The subscription add posts without a pre-check and lets the broker's
	// AlreadyExists code report the skip. Carried over from the previous
	// implementation, whose test asserts that no GET is issued on the live path.
	"msg_vpn_queue_subscription": func(d *objectDesc) {
		d.AddMode = addOptimistic
	},

	// An ACL exception's key is (topicSyntax, topic). Messages name the topic,
	// which is what an operator recognises, not the syntax.
	"msg_vpn_acl_profile_publish_topic_exception": func(d *objectDesc) {
		d.DisplayKey = 1
	},
	"msg_vpn_acl_profile_subscribe_topic_exception": func(d *objectDesc) {
		d.DisplayKey = 1
	},
}

// overrideKeysAreReal is asserted by a test rather than at init: an entry that
// matches no descriptor would silently stop applying, exactly the failure mode
// the generator's checkLegacyMatched guards against on its side of the fence.

// queueDerivedFields sets the two queue attributes the broker requires to be
// consistent with the values an operator actually supplies, so a template can
// state an intent ("keep messages for an hour", "give up after 5 attempts")
// without also having to set the matching enable flag.
//
// Behaviour is carried over unchanged from the previous implementation:
//
//   - maxTtl > 0 enables TTL enforcement; 0 disables it.
//   - maxRedeliveryCount == -1 is a sentinel meaning "disable redelivery",
//     which the broker expresses as a count of 0 with the flag off. Any other
//     non-zero count enables redelivery; 0 means unlimited attempts, which also
//     leaves the flag on.
//
// The values have already been coerced to int by the time a hook runs, so a
// failed assertion here would mean the descriptor declared the wrong kind; the
// checked form leaves the payload untouched rather than panicking.
func queueDerivedFields(_, payload map[string]any) error {
	if n, ok := payload["maxTtl"].(int); ok {
		payload["respectTtlEnabled"] = n > 0
	}
	if n, ok := payload["maxRedeliveryCount"].(int); ok {
		if n == -1 {
			payload["maxRedeliveryCount"] = 0
			payload["redeliveryEnabled"] = false
		} else {
			payload["redeliveryEnabled"] = n != 0
		}
	}
	return nil
}
