package httpcloak

import "testing"

// WithoutCookieJar() must set the sessionConfig flag so the lower layers
// (session.Session) skip jar injection on requests AND skip Set-Cookie
// extraction from responses. Static sanity check; live flow is exercised
// by the per-binding integration scripts in internal_docs/.
func TestWithoutCookieJar_FlagPropagation(t *testing.T) {
	cfg := &sessionConfig{}
	if cfg.withoutCookieJar {
		t.Fatalf("zero-value sessionConfig.withoutCookieJar = true, want false")
	}
	WithoutCookieJar()(cfg)
	if !cfg.withoutCookieJar {
		t.Fatalf("WithoutCookieJar() did not set the flag, withoutCookieJar = false")
	}
}

// Default (no option applied) keeps the jar enabled. Pins the no-surprise
// contract: existing callers see no behaviour change.
func TestWithoutCookieJar_DefaultIsJarEnabled(t *testing.T) {
	cfg := &sessionConfig{}
	if cfg.withoutCookieJar {
		t.Fatalf("default sessionConfig has cookie jar disabled, want enabled")
	}
}
