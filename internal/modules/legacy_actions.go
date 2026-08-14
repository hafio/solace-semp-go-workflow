package modules

// Action names are canonical -- the Terraform resource name -- but the ten
// families carried over from the previous implementation also answer to their
// original short prefix. Existing templates and customer configs name those
// prefixes directly (`module: "queue.add"`), so dropping them would break every
// one of them at runtime with "Unknown module" and no compile-time signal.
//
// This is the object-name half of the dual-accept contract; the attribute half
// lives in canon.go. Both exist so a config written against the previous
// implementation runs unchanged.
//
// Nothing new belongs here. A legacy name is a backward-compatibility debt, not a
// convenience: an object added after this point has one name, its canonical one.
var legacyActionPrefixes = map[string]string{
	"msg_vpn_queue":                                 "queue",
	"msg_vpn_queue_subscription":                    "q_sub",
	"msg_vpn_rest_delivery_point":                   "rdp",
	"msg_vpn_rest_delivery_point_rest_consumer":     "rdp_rc",
	"msg_vpn_rest_delivery_point_queue_binding":     "rdp_qb",
	"msg_vpn_client_profile":                        "client_profile",
	"msg_vpn_client_username":                       "client_username",
	"msg_vpn_acl_profile":                           "acl_profile",
	"msg_vpn_acl_profile_publish_topic_exception":   "acl_publish_exception",
	"msg_vpn_acl_profile_subscribe_topic_exception": "acl_subscribe_exception",
}

// legacyRegistry resolves the old short action names. It is deliberately separate
// from registry: Get consults it, but List and Info do not, so the documented
// surface and the generated module reference stay canonical-only and a legacy
// name never appears as though it were a second module.
var legacyRegistry = map[string]Module{}
