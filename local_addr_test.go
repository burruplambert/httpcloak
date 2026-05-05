package httpcloak

import (
	"net"
	"testing"
)

// WithLocalAddrIP is the net.IP-typed alias for WithLocalAddress. It must
// produce the same on-wire localAddr string as the existing string form so
// callers can mix the two without surprise.
func TestWithLocalAddrIP_ParityWithStringForm(t *testing.T) {
	cases := []struct {
		name string
		ip   net.IP
	}{
		{"ipv4", net.ParseIP("192.0.2.1")},
		{"ipv6", net.ParseIP("2001:db8::1234")},
		{"ipv4-mapped-ipv6", net.ParseIP("::ffff:192.0.2.5")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfgIP := &sessionConfig{}
			WithLocalAddrIP(tc.ip)(cfgIP)

			cfgStr := &sessionConfig{}
			WithLocalAddress(tc.ip.String())(cfgStr)

			if cfgIP.localAddr != cfgStr.localAddr {
				t.Errorf("ip-form vs string-form drift: ip=%q str=%q", cfgIP.localAddr, cfgStr.localAddr)
			}
		})
	}
}

// nil net.IP must NOT clobber an existing value. Callers chaining options
// conditionally (`opts = append(opts, WithLocalAddrIP(maybeNil))`) shouldn't
// be punished for passing a possibly-nil value.
func TestWithLocalAddrIP_NilIsNoOp(t *testing.T) {
	cfg := &sessionConfig{}
	WithLocalAddress("192.0.2.99")(cfg)
	WithLocalAddrIP(nil)(cfg)
	if cfg.localAddr != "192.0.2.99" {
		t.Errorf("nil net.IP clobbered prior localAddr: got %q, want 192.0.2.99", cfg.localAddr)
	}
}
