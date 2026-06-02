package session

import "testing"

// Locks Chrome's default strict-origin-when-cross-origin Referer policy for the
// redirect path (issue #70): full URL same-origin, origin-only cross-origin
// (trailing slash), omitted on an https->http downgrade.
func TestRedirectReferer(t *testing.T) {
	cases := []struct {
		name        string
		prevURL     string
		downgrade   bool
		crossOrigin bool
		want        string
	}{
		{"same-origin keeps full URL", "https://example.com/a/b?q=1", false, false, "https://example.com/a/b?q=1"},
		{"cross-origin -> origin only", "https://example.com/a/b?q=1", false, true, "https://example.com/"},
		{"cross-origin non-default port kept", "https://example.com:8443/x", false, true, "https://example.com:8443/"},
		{"cross-origin default https port dropped", "https://example.com:443/x", false, true, "https://example.com/"},
		{"https->http downgrade omits", "https://example.com/a", true, true, ""},
		{"http same-origin keeps full URL", "http://example.com:80/a", false, false, "http://example.com:80/a"},
		{"http cross-origin default port dropped", "http://example.com:80/a", false, true, "http://example.com/"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := redirectReferer(c.prevURL, c.downgrade, c.crossOrigin)
			if got != c.want {
				t.Fatalf("redirectReferer(%q, downgrade=%v, cross=%v) = %q, want %q", c.prevURL, c.downgrade, c.crossOrigin, got, c.want)
			}
		})
	}
}
