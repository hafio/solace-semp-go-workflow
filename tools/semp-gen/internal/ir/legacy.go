package ir

// This file is the backward-compatibility contract. Everything here exists
// because the previous implementation behaved this way and existing workflow
// templates and customer configs depend on it -- not because the specification
// says so. It is hand-maintained and small on purpose: the ten families carried
// over need it, and nothing else should ever be added without a stated reason.
//
// Attribute data (names, types, enums, defaults, descriptions) is NOT pinned
// here. It comes from the specification, so these ten objects gain their full
// attribute surface like any other: a queue goes from the ten parameters the old
// tool exposed to all 34 the broker accepts.

// legacyOverlay is applied after extraction, keyed by registry prefix.
type legacyOverlay struct {
	// FriendlyName and Article are how the object reads in result messages. The
	// article is stated because pronunciation decides it and spelling does not:
	// "an RDP" is spelled out letter by letter, "a REST consumer" is said as a
	// word, and both are all-caps.
	FriendlyName       string
	Article            string
	ParentFriendlyName string

	// Messages pin wording that differs from the runtime's shape defaults, keyed
	// by msgSet field name. A subscription is "added to" a queue rather than
	// "created on" it, an ACL exception is "removed from" a profile, and some
	// legacy failures name no object at all.
	Messages map[string]string

	// KeyDefaults let a workflow omit a key that historically defaulted. An ACL
	// exception's topic syntax is the only case: templates name the topic alone.
	KeyDefaults map[string]string

	// KeyDescs pin the wording of a key's documented description.
	KeyDescs map[string]string

	// DisplayKey selects which part of a composite key appears in messages.
	DisplayKey int

	// SkipAttributes are derived by a payload hook in the runtime's overrides.go,
	// so offering them as settable parameters would invite a caller to set a
	// value the hook then overwrites.
	SkipAttributes []string

	// AttrDescs pin an attribute description where the legacy wording documents
	// tool behaviour the specification cannot know about.
	AttrDescs map[string]string
}

// nested is the wording shared by the objects that read as "added to" and
// "removed from" their parent rather than "created on" and "deleted from" it.
func addedRemovedMsgs(nameInErrors bool) map[string]string {
	m := map[string]string{
		"WouldCreate": "Would add {obj} '{name}' to {parent} '{pname}'",
		"WouldDelete": "Would remove {obj} '{name}' from {parent} '{pname}'",
		"Created":     "{Obj} '{name}' added to {parent} '{pname}'",
		"Deleted":     "{Obj} '{name}' removed from {parent} '{pname}'",
	}
	if nameInErrors {
		m["CreateErr"] = "Failed to add {obj} '{name}' to {parent} '{pname}': {err}"
		m["DeleteErr"] = "Failed to remove {obj} '{name}' from {parent} '{pname}': {err}"
	} else {
		m["CreateErr"] = "Failed to add {obj}: {err}"
		m["DeleteErr"] = "Failed to remove {obj}: {err}"
	}
	return m
}

// pluralMissingArg is the wording used by objects whose legacy message named
// every required key together, so the plural reads correctly whichever is absent.
var pluralMissingArg = map[string]string{"MissingArg": "Missing required args: {arg}"}

var legacyOverlays = map[string]legacyOverlay{
	"MsgVpnQueue": {
		FriendlyName: "queue",
		// respectTtlEnabled and redeliveryEnabled are derived from maxTtl and
		// maxRedeliveryCount by the queue hook.
		SkipAttributes: []string{"respectTtlEnabled", "redeliveryEnabled"},
		KeyDescs:       map[string]string{"queueName": "Name of the queue"},
		AttrDescs: map[string]string{
			"maxTtl": "Maximum time-to-live for messages in seconds. 0 disables TTL enforcement; " +
				"any positive value enables it automatically.",
			"maxRedeliveryCount": "Maximum redelivery attempts before routing to the dead message queue. " +
				"0 means unlimited; -1 disables redelivery entirely.",
		},
	},

	"MsgVpnQueueSubscription": {
		FriendlyName:       "subscription",
		ParentFriendlyName: "queue",
		Messages:           merge(addedRemovedMsgs(false), pluralMissingArg),
		KeyDescs: map[string]string{
			"queueName":         "Name of the queue to subscribe",
			"subscriptionTopic": "Topic string to subscribe to (wildcards supported, e.g. SITEA/APP/>)",
		},
	},

	"MsgVpnRestDeliveryPoint": {
		FriendlyName: "RDP",
		Article:      "an",
		KeyDescs:     map[string]string{"restDeliveryPointName": "Name of the REST Delivery Point"},
	},

	"MsgVpnRestDeliveryPointRestConsumer": {
		FriendlyName:       "REST consumer",
		ParentFriendlyName: "RDP",
		Messages:           pluralMissingArg,
		KeyDescs: map[string]string{
			"restDeliveryPointName": "Name of the parent REST Delivery Point",
			"restConsumerName":      "Name of the REST consumer",
		},
	},

	"MsgVpnRestDeliveryPointQueueBinding": {
		FriendlyName:       "queue binding",
		ParentFriendlyName: "RDP",
		Messages:           pluralMissingArg,
		KeyDescs: map[string]string{
			"restDeliveryPointName": "Name of the REST Delivery Point",
			"queueBindingName":      "Name of the queue to bind",
		},
	},

	"MsgVpnClientProfile": {
		FriendlyName: "client profile",
		KeyDescs:     map[string]string{"clientProfileName": "Name of the client profile"},
	},

	"MsgVpnClientUsername": {
		FriendlyName: "client username",
		KeyDescs:     map[string]string{"clientUsername": "The client username"},
	},

	"MsgVpnAclProfile": {
		FriendlyName: "ACL profile",
		Article:      "an",
		KeyDescs:     map[string]string{"aclProfileName": "Name of the ACL profile"},
	},

	"MsgVpnAclProfilePublishTopicException": {
		FriendlyName:       "publish topic exception",
		ParentFriendlyName: "ACL profile",
		Messages:           addedRemovedMsgs(true),
		KeyDefaults:        map[string]string{"publishTopicExceptionSyntax": "smf"},
		KeyDescs: map[string]string{
			"aclProfileName":              "Name of the ACL profile",
			"publishTopicExceptionSyntax": "Syntax of the topic",
			"publishTopicException":       "The topic for the exception (may include wildcards)",
		},
		DisplayKey: 1,
	},

	"MsgVpnAclProfileSubscribeTopicException": {
		FriendlyName:       "subscribe topic exception",
		ParentFriendlyName: "ACL profile",
		Messages:           addedRemovedMsgs(true),
		KeyDefaults:        map[string]string{"subscribeTopicExceptionSyntax": "smf"},
		KeyDescs: map[string]string{
			"aclProfileName":                "Name of the ACL profile",
			"subscribeTopicExceptionSyntax": "Syntax of the topic",
			"subscribeTopicException":       "The topic for the exception (may include wildcards)",
		},
		DisplayKey: 1,
	},
}

func merge(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// applyLegacy overlays the backward-compatibility contract onto an extracted
// object. Objects with no overlay -- the other 76 -- are untouched and take the
// spec-derived naming and the runtime's default wording.
func applyLegacy(o *Object) {
	// Keyed by Definition, not Prefix: the definition name is Solace's own stable
	// identifier, so this table survives a change to how prefixes are derived. It
	// did not survive the last one silently -- checkLegacyMatched would have
	// failed the build -- but keying off the stable name means it will not have to.
	ov, ok := legacyOverlays[o.Definition]
	if !ok {
		return
	}
	if ov.FriendlyName != "" {
		o.FriendlyName = ov.FriendlyName
	}
	if ov.Article != "" {
		o.Article = ov.Article
	}
	if ov.ParentFriendlyName != "" {
		o.ParentFriendlyName = ov.ParentFriendlyName
	}
	o.Messages = ov.Messages
	o.DisplayKey = ov.DisplayKey
	o.SkipHookDerived = ov.SkipAttributes

	for i := range o.OwnKey {
		if def, ok := ov.KeyDefaults[o.OwnKey[i].Name]; ok {
			o.OwnKey[i].Default = def
		}
		if desc, ok := ov.KeyDescs[o.OwnKey[i].Name]; ok {
			o.OwnKey[i].Desc = desc
		}
	}
	for h := range o.Parents {
		for i := range o.Parents[h].Keys {
			if desc, ok := ov.KeyDescs[o.Parents[h].Keys[i].Name]; ok {
				o.Parents[h].Keys[i].Desc = desc
			}
		}
	}

	kept := o.Attributes[:0]
	skip := map[string]bool{}
	for _, s := range ov.SkipAttributes {
		skip[s] = true
	}
	for _, a := range o.Attributes {
		if skip[a.Name] {
			continue
		}
		if desc, ok := ov.AttrDescs[a.Name]; ok {
			a.Desc = desc
		}
		kept = append(kept, a)
	}
	o.Attributes = kept
}

// LegacyDefinitions is the set of definitions the overlay pins, so the generator
// can report that each was actually matched by a real object.
func LegacyDefinitions() []string {
	out := make([]string, 0, len(legacyOverlays))
	for p := range legacyOverlays {
		out = append(out, p)
	}
	return out
}
