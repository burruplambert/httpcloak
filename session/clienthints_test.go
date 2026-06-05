package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/sardanioss/httpcloak/protocol"
	"github.com/sardanioss/httpcloak/transport"
)

// chHarness is a local TLS+H2 server that advertises Accept-CH and records the
// sec-ch-* (and User-Agent) headers it receives per request.
type chHarness struct {
	srv  *httptest.Server
	mu   sync.Mutex
	hits []http.Header
}

func newCHHarness(t *testing.T) *chHarness {
	h := &chHarness{}
	h.srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		hdr := http.Header{}
		for k, v := range r.Header {
			hdr[http.CanonicalHeaderKey(k)] = v
		}
		h.hits = append(h.hits, hdr)
		h.mu.Unlock()
		w.Header().Set("Accept-CH", "sec-ch-ua-full-version-list, sec-ch-ua-arch, sec-ch-ua-platform-version, sec-ch-ua-bitness, sec-ch-ua-model")
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	h.srv.EnableHTTP2 = true // streaming has no H1 fallback; serve H2 ALPN
	h.srv.StartTLS()
	t.Cleanup(h.srv.Close)
	return h
}

func (h *chHarness) hit(i int) http.Header {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hits[i]
}

func (h *chHarness) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.hits)
}

func chSession(preset string, mutate func(*protocol.SessionConfig)) *Session {
	cfg := &protocol.SessionConfig{
		Preset:             preset,
		Timeout:            10,
		InsecureSkipVerify: true,
	}
	if mutate != nil {
		mutate(cfg)
	}
	return NewSession("", cfg)
}

func anySecCh(h http.Header) bool {
	for k := range h {
		if strings.HasPrefix(k, "Sec-Ch-") {
			return true
		}
	}
	return false
}

func getBuffered(t *testing.T, s *Session, url string) {
	t.Helper()
	resp, err := s.Request(context.Background(), &transport.Request{Method: "GET", URL: url})
	if err != nil {
		t.Fatalf("buffered request: %v", err)
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
}

func getStream(t *testing.T, s *Session, url string) {
	t.Helper()
	resp, err := s.RequestStream(context.Background(), &transport.Request{Method: "GET", URL: url})
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	resp.Close()
}

// TestClientHintsCoherentAfterAcceptCH locks the headline fix: after a host
// advertises Accept-CH, the emitted full-version-list agrees with sec-ch-ua
// (major 149, matching GREASE token) instead of the old stale 145, and Linux
// sends an empty platform version.
func TestClientHintsCoherentAfterAcceptCH(t *testing.T) {
	h := newCHHarness(t)
	s := chSession("chrome-149-linux", nil)
	defer s.Close()

	getBuffered(t, s, h.srv.URL+"/p") // learns Accept-CH
	getBuffered(t, s, h.srv.URL+"/p") // sends high-entropy

	req2 := h.hit(1)
	fvl := req2.Get("Sec-Ch-Ua-Full-Version-List")
	if !strings.Contains(fvl, `"Google Chrome";v="149.`) {
		t.Errorf("full-version-list not coherent with Chrome 149: %q", fvl)
	}
	if !strings.Contains(fvl, `"Not)A;Brand";v="24.`) {
		t.Errorf("full-version-list GREASE token does not match sec-ch-ua: %q", fvl)
	}
	if strings.Contains(fvl, "145") {
		t.Errorf("full-version-list still carries the stale 145: %q", fvl)
	}
	if pv := req2.Get("Sec-Ch-Ua-Platform-Version"); pv != `""` {
		t.Errorf("Linux platform-version should be empty-quoted, got %q", pv)
	}
}

// TestClientHintsOverrideDeterministic locks the determinism fix: a user-supplied
// high-entropy hint always wins, never racing the injected value.
func TestClientHintsOverrideDeterministic(t *testing.T) {
	for i := 0; i < 5; i++ {
		h := newCHHarness(t)
		s := chSession("chrome-149-linux", nil)
		getBuffered(t, s, h.srv.URL+"/p") // learn Accept-CH
		resp, err := s.Request(context.Background(), &transport.Request{
			Method:  "GET",
			URL:     h.srv.URL + "/p",
			Headers: map[string][]string{"sec-ch-ua-full-version-list": {`"Mine";v="9.9.9.9"`}},
		})
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if resp.Body != nil {
			resp.Body.Close()
		}
		got := h.hit(1).Get("Sec-Ch-Ua-Full-Version-List")
		if got != `"Mine";v="9.9.9.9"` {
			t.Fatalf("iter %d: user override lost, got %q", i, got)
		}
		s.Close()
	}
}

// TestClientHintsFullStrip locks the full opt-out: zero sec-ch-* on the wire
// (buffered and streaming), UA still present.
func TestClientHintsFullStrip(t *testing.T) {
	t.Run("buffered", func(t *testing.T) {
		h := newCHHarness(t)
		s := chSession("chrome-149-linux", func(c *protocol.SessionConfig) { c.WithoutClientHints = true })
		defer s.Close()
		getBuffered(t, s, h.srv.URL+"/p")
		getBuffered(t, s, h.srv.URL+"/p")
		for i := 0; i < h.count(); i++ {
			if anySecCh(h.hit(i)) {
				t.Errorf("buffered req %d leaked sec-ch-*: %v", i, h.hit(i))
			}
		}
		if h.hit(0).Get("User-Agent") == "" {
			t.Error("UA should still be sent under full strip")
		}
	})
	t.Run("streaming", func(t *testing.T) {
		h := newCHHarness(t)
		s := chSession("chrome-149-linux", func(c *protocol.SessionConfig) { c.WithoutClientHints = true })
		defer s.Close()
		getStream(t, s, h.srv.URL+"/p")
		getStream(t, s, h.srv.URL+"/p")
		for i := 0; i < h.count(); i++ {
			if anySecCh(h.hit(i)) {
				t.Errorf("stream req %d leaked sec-ch-*: %v", i, h.hit(i))
			}
		}
	})
	t.Run("per-request", func(t *testing.T) {
		h := newCHHarness(t)
		s := chSession("chrome-149-linux", nil)
		defer s.Close()
		resp, err := s.Request(context.Background(), &transport.Request{
			Method: "GET", URL: h.srv.URL + "/p", DisableClientHints: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Body != nil {
			resp.Body.Close()
		}
		if anySecCh(h.hit(0)) {
			t.Errorf("per-request DisableClientHints leaked sec-ch-*: %v", h.hit(0))
		}
	})
}

// TestClientHintsHighEntropyOnly locks the partial opt-out: the trio stays, the
// high-entropy hints are suppressed.
func TestClientHintsHighEntropyOnly(t *testing.T) {
	h := newCHHarness(t)
	s := chSession("chrome-149-linux", func(c *protocol.SessionConfig) { c.WithoutHighEntropyClientHints = true })
	defer s.Close()
	getBuffered(t, s, h.srv.URL+"/p")
	getBuffered(t, s, h.srv.URL+"/p")
	req2 := h.hit(1)
	if req2.Get("Sec-Ch-Ua") == "" {
		t.Error("trio should be present under high-entropy opt-out")
	}
	if fvl := req2.Get("Sec-Ch-Ua-Full-Version-List"); fvl != "" {
		t.Errorf("high-entropy hint leaked under opt-out: %q", fvl)
	}
}

// TestClientHintsStreamingParity locks parity: streaming emits high-entropy hints
// after Accept-CH, matching the buffered path.
func TestClientHintsStreamingParity(t *testing.T) {
	h := newCHHarness(t)
	s := chSession("chrome-149-linux", nil)
	defer s.Close()
	getStream(t, s, h.srv.URL+"/p") // learns Accept-CH from the stream response
	getStream(t, s, h.srv.URL+"/p")
	fvl := h.hit(1).Get("Sec-Ch-Ua-Full-Version-List")
	if !strings.Contains(fvl, `"Google Chrome";v="149.`) {
		t.Errorf("streaming did not emit coherent high-entropy hints after Accept-CH: %q", fvl)
	}
}
