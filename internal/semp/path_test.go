package semp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath(t *testing.T) {
	cases := []struct {
		name string
		segs []Seg
		want string
	}{
		{
			name: "no segments addresses the broker singleton",
			segs: nil,
			want: "",
		},
		{
			name: "bare collection hop is the POST target",
			segs: []Seg{VPNSeg("vpn1"), {Literal: "queues"}},
			want: "msgVpns/vpn1/queues",
		},
		{
			name: "single simple key",
			segs: []Seg{VPNSeg("vpn1"), {Literal: "queues", Parts: []string{"orders"}}},
			want: "msgVpns/vpn1/queues/orders",
		},
		{
			name: "nested child under a parent item",
			segs: []Seg{
				VPNSeg("vpn1"),
				{Literal: "restDeliveryPoints", Parts: []string{"rdp1"}},
				{Literal: "queueBindings", Parts: []string{"orders"}},
			},
			want: "msgVpns/vpn1/restDeliveryPoints/rdp1/queueBindings/orders",
		},
		{
			name: "two-part composite key joins with a comma",
			segs: []Seg{
				VPNSeg("vpn1"),
				{Literal: "bridges", Parts: []string{"b1", "primary"}},
			},
			want: "msgVpns/vpn1/bridges/b1,primary",
		},
		{
			name: "three-part composite key under a composite parent",
			segs: []Seg{
				VPNSeg("vpn1"),
				{Literal: "bridges", Parts: []string{"b1", "primary"}},
				{Literal: "remoteMsgVpns", Parts: []string{"vpn2", "10.0.0.1:55555", "eth0"}},
			},
			want: "msgVpns/vpn1/bridges/b1,primary/remoteMsgVpns/vpn2,10.0.0.1%3A55555,eth0",
		},
		{
			name: "broker-global object omits the VPN hop",
			segs: []Seg{{Literal: "dmrClusters", Parts: []string{"cluster1"}}, {Literal: "links", Parts: []string{"node1"}}},
			want: "dmrClusters/cluster1/links/node1",
		},
		{
			name: "each key part is encoded independently",
			segs: []Seg{
				VPNSeg("vpn1"),
				{Literal: "aclProfiles", Parts: []string{"P"}},
				{Literal: "publishTopicExceptions", Parts: []string{"smf", "SITEA/>"}},
			},
			want: "msgVpns/vpn1/aclProfiles/P/publishTopicExceptions/smf,SITEA%2F%3E",
		},
		{
			name: "a comma inside a key value cannot add a path part",
			segs: []Seg{VPNSeg("vpn1"), {Literal: "queues", Parts: []string{"a,b"}}},
			want: "msgVpns/vpn1/queues/a%2Cb",
		},
		{
			name: "a slash inside a key value cannot add a path segment",
			segs: []Seg{VPNSeg("vpn1"), {Literal: "queues", Parts: []string{"a/../b"}}},
			want: "msgVpns/vpn1/queues/a%2F..%2Fb",
		},
		{
			name: "VPN name is encoded too",
			segs: []Seg{VPNSeg("my vpn"), {Literal: "queues", Parts: []string{"q"}}},
			want: "msgVpns/my%20vpn/queues/q",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Path(tc.segs...))
		})
	}
}

func TestResolveVPN(t *testing.T) {
	cases := []struct {
		name      string
		explicit  string
		defaulted string
		want      string
	}{
		{"explicit wins", "other-vpn", "default-vpn", "other-vpn"},
		{"falls back to the default", "", "default-vpn", "default-vpn"},
		{"both empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ResolveVPN(tc.explicit, tc.defaulted))
		})
	}
}
