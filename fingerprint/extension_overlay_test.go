package fingerprint

import (
	"bytes"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	tls "github.com/sardanioss/utls"
)

// overlayBase is the preset the byte-level tests build on.
//
// It is Firefox rather than Chrome because a Chrome parrot cannot be compared
// byte for byte at all: utlsIdToSpec shuffles the Chrome literal with fresh
// randomness before UTLSIdToSpecWithSeed applies the seeded shuffle, so two
// specs built from one ClientHelloID and one seed carry their extensions in
// different orders. Firefox does not permute, and its hello is far enough over
// the padding band that adding an extension cannot change a padding length, so
// two hellos from it differ only where their specs do. TestOverlayOnAPermutingBase
// covers Chrome, comparing the extensions as a set.
//
// Firefox 133 sends record_size_limit and does not send
// signed_certificate_timestamp, which makes one a clean drop and the other a
// clean add.
const overlayBase = "firefox-133-windows"

const (
	extSCT             = 18
	extPadding         = 21
	extEMS             = 23
	extCompressCert    = 27
	extRecordSizeLimit = 28
	extKeyShare        = 51
	extSessionTicket   = 35
	extTrustAnchors    = 0xCA34
	extALPS            = 17613
	extStatusRequest   = 5
	extRenegotiation   = 65281
	extECH             = 0xFE0D
)

// carriesFreshKeyMaterial reports whether an extension's body is redrawn for
// every hello no matter what the spec says.
//
// key_share holds a public key generated per handshake, and the GREASE ECH
// payload is random padding sized to a real one. Neither reads the client
// random's stream, so pinning Config.Rand does not pin them, and comparing
// their bodies across two hellos would compare fresh key material. Their
// codepoint, position and length are still compared, which is everything about
// them the overlay could have changed.
func carriesFreshKeyMaterial(id uint16) bool {
	return id == extKeyShare || id == extECH
}

// wireHello is a serialized ClientHello taken apart far enough to compare two
// of them field by field.
type wireHello struct {
	LegacyVersion []byte
	Random        []byte
	SessionID     []byte
	CipherSuites  []byte
	Compression   []byte
	Extensions    []wireExtension
}

// wireExtension is one extension exactly as it appears on the wire, codepoint
// and body together, so comparing two of them compares their bytes.
type wireExtension struct {
	ID    uint16
	Bytes []byte
}

func (e wireExtension) String() string {
	return fmt.Sprintf("%d(0x%04x)/%dB", e.ID, e.ID, len(e.Bytes))
}

// helloBytes serializes the ClientHello a spec produces.
//
// Every source of per-handshake randomness is pinned to the same stream, so two
// hellos built this way differ only where their specs differ. Without that the
// comparison is impossible: the client random, the session id, the GREASE
// values and the key-share keys are all drawn fresh, and a byte diff would be
// noise from end to end.
func helloBytes(t *testing.T, spec *tls.ClientHelloSpec) []byte {
	t.Helper()
	cfg := &tls.Config{
		ServerName: "example.com",
		Rand:       rand.NewChaCha8([32]byte{'o', 'v', 'e', 'r', 'l', 'a', 'y'}),
	}
	conn := tls.UClient(nil, cfg, tls.HelloCustom)
	if err := conn.ApplyPreset(spec); err != nil {
		t.Fatalf("apply spec: %v", err)
	}
	if err := conn.BuildHandshakeStateWithoutSession(); err != nil {
		t.Fatalf("build hello: %v", err)
	}
	return conn.HandshakeState.Hello.Raw
}

// parseHello takes a serialized ClientHello apart.
func parseHello(t *testing.T, raw []byte) wireHello {
	t.Helper()
	if len(raw) < 4 || raw[0] != 1 {
		t.Fatalf("not a client hello: % x", raw[:min(8, len(raw))])
	}
	b := raw[4:] // past the handshake type and 24-bit length

	var h wireHello
	take := func(n int, what string) []byte {
		if len(b) < n {
			t.Fatalf("client hello truncated in %s", what)
		}
		out := b[:n]
		b = b[n:]
		return out
	}
	h.LegacyVersion = take(2, "legacy_version")
	h.Random = take(32, "random")
	h.SessionID = take(1+int(b[0]), "session id")
	h.CipherSuites = take(2+(int(b[0])<<8|int(b[1])), "cipher suites")
	h.Compression = take(1+int(b[0]), "compression methods")

	total := int(b[0])<<8 | int(b[1])
	b = b[2:]
	if len(b) < total {
		t.Fatalf("client hello truncated in the extension block")
	}
	for b = b[:total]; len(b) >= 4; {
		id := uint16(b[0])<<8 | uint16(b[1])
		size := int(b[2])<<8 | int(b[3])
		if len(b) < 4+size {
			t.Fatalf("client hello truncated in extension %d", id)
		}
		h.Extensions = append(h.Extensions, wireExtension{ID: id, Bytes: b[:4+size]})
		b = b[4+size:]
	}
	return h
}

func overlaySpec(t *testing.T, overlay ExtensionOverlay) *tls.ClientHelloSpec {
	t.Helper()
	p := GetStrict(overlayBase)
	if p == nil {
		t.Fatalf("preset %q is not registered", overlayBase)
	}
	spec, err := SpecForWithOverlay(p.ClientHelloID, 1, p.SignatureAlgorithms, p.TrustAnchors, overlay)
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	return spec
}

// The whole point of the overlay is that it is the only difference on the wire.
// A caller reaches for it precisely because a JA3 or a captured hello would
// replace everything the ClientHelloID gets right, so a change anywhere outside
// the named extensions is the failure this test exists to catch.
func TestExtensionOverlayChangesOnlyTheNamedExtensions(t *testing.T) {
	base := parseHello(t, helloBytes(t, overlaySpec(t, ExtensionOverlay{})))
	over := parseHello(t, helloBytes(t, overlaySpec(t, ExtensionOverlay{
		Add:  []uint16{extSCT},
		Drop: []uint16{extRecordSizeLimit},
	})))

	for _, f := range []struct {
		name      string
		got, want []byte
	}{
		{"legacy_version", over.LegacyVersion, base.LegacyVersion},
		{"random", over.Random, base.Random},
		{"session id", over.SessionID, base.SessionID},
		{"cipher suites", over.CipherSuites, base.CipherSuites},
		{"compression methods", over.Compression, base.Compression},
	} {
		if !bytes.Equal(f.got, f.want) {
			t.Errorf("%s changed: got % x, want % x", f.name, f.got, f.want)
		}
	}

	// Everything the overlay did not name has to survive with its bytes and its
	// position relative to the others intact.
	var wantExts []wireExtension
	for _, e := range base.Extensions {
		if e.ID != extRecordSizeLimit {
			wantExts = append(wantExts, e)
		}
	}
	var gotExts []wireExtension
	var added []wireExtension
	for _, e := range over.Extensions {
		if e.ID == extSCT {
			added = append(added, e)
			continue
		}
		gotExts = append(gotExts, e)
	}

	if len(gotExts) != len(wantExts) {
		t.Fatalf("after accounting for the overlay the hello carries %d extensions, want %d\n got: %v\nwant: %v",
			len(gotExts), len(wantExts), gotExts, wantExts)
	}
	for i := range gotExts {
		if gotExts[i].ID != wantExts[i].ID {
			t.Fatalf("extension %d is %s, want %s: the overlay reordered the list",
				i, gotExts[i], wantExts[i])
		}
		if len(gotExts[i].Bytes) != len(wantExts[i].Bytes) {
			t.Errorf("extension %s changed length, want %s", gotExts[i], wantExts[i])
			continue
		}
		if carriesFreshKeyMaterial(gotExts[i].ID) {
			continue
		}
		if !bytes.Equal(gotExts[i].Bytes, wantExts[i].Bytes) {
			t.Errorf("extension %s changed on the wire:\n got % x\nwant % x",
				gotExts[i], gotExts[i].Bytes, wantExts[i].Bytes)
		}
	}

	if len(added) != 1 {
		t.Fatalf("the hello carries %d signed_certificate_timestamp extensions, want 1", len(added))
	}
	// Empty, as every client that sends it does: codepoint then a zero length.
	if want := []byte{0x00, 0x12, 0x00, 0x00}; !bytes.Equal(added[0].Bytes, want) {
		t.Errorf("signed_certificate_timestamp went out as % x, want % x", added[0].Bytes, want)
	}
	for _, e := range over.Extensions {
		if e.ID == extRecordSizeLimit {
			t.Error("record_size_limit is still on the wire after being dropped")
		}
	}
}

// An added extension lands ahead of the trailing run that may not move, for the
// reasons ApplyTrustAnchors gives: GREASE brackets the list, padding sizes the
// record and RFC 8446 4.2.11 puts pre_shared_key last.
func TestApplyExtensionOverlayInsertsBeforeThePinnedTail(t *testing.T) {
	exts := append(tailSpec(), &tls.FakePreSharedKeyExtension{})
	ApplyExtensionOverlay(&exts, ExtensionOverlay{Add: []uint16{extRecordSizeLimit}})

	at := -1
	for i, e := range exts {
		if _, ok := e.(*tls.FakeRecordSizeLimitExtension); ok {
			at = i
		}
	}
	if at != 4 {
		t.Fatalf("record_size_limit landed at index %d, want 4, ahead of the trailing "+
			"GREASE, padding and pre_shared_key", at)
	}
	if _, ok := exts[len(exts)-1].(*tls.FakePreSharedKeyExtension); !ok {
		t.Error("pre_shared_key is no longer last, which RFC 8446 4.2.11 requires")
	}
}

// A GREASE extension reports no codepoint, so a drop cannot remove one by
// naming the value it happens to be carrying. Chrome is recognised by that
// bracket, and an overlay that could delete it would be a way to break the
// hello rather than to describe a client.
func TestApplyExtensionOverlayLeavesGREASEAlone(t *testing.T) {
	exts := tailSpec()
	before := len(exts)
	ApplyExtensionOverlay(&exts, ExtensionOverlay{Drop: []uint16{0x0a0a, 0x1a1a}})
	if len(exts) != before {
		t.Fatalf("a GREASE codepoint in drop removed %d extensions", before-len(exts))
	}
	if _, ok := exts[0].(*tls.UtlsGREASEExtension); !ok {
		t.Error("the leading GREASE extension is gone")
	}
}

// Naming something already true of the hello changes nothing, in either
// direction. Both are the caller describing a client the base already matches,
// and neither should make the hello worse.
func TestApplyExtensionOverlayNoOps(t *testing.T) {
	base := parseHello(t, helloBytes(t, overlaySpec(t, ExtensionOverlay{})))

	for name, overlay := range map[string]ExtensionOverlay{
		"add an extension the hello already sends": {Add: []uint16{extSessionTicket}},
		"drop one it does not send":                {Drop: []uint16{extCompressCert}},
		"empty":                                    {},
	} {
		t.Run(name, func(t *testing.T) {
			got := parseHello(t, helloBytes(t, overlaySpec(t, overlay)))
			if len(got.Extensions) != len(base.Extensions) {
				t.Fatalf("the hello carries %d extensions, want the base's %d",
					len(got.Extensions), len(base.Extensions))
			}
			for i := range got.Extensions {
				if got.Extensions[i].ID != base.Extensions[i].ID ||
					len(got.Extensions[i].Bytes) != len(base.Extensions[i].Bytes) {
					t.Fatalf("extension %d changed: got %s, want %s",
						i, got.Extensions[i], base.Extensions[i])
				}
				if carriesFreshKeyMaterial(got.Extensions[i].ID) {
					continue
				}
				if !bytes.Equal(got.Extensions[i].Bytes, base.Extensions[i].Bytes) {
					t.Fatalf("extension %s changed on the wire", got.Extensions[i])
				}
			}
		})
	}
}

// The overlay runs after the trust-anchor insert, so the two compose rather
// than one deciding where the other lands.
func TestApplyExtensionOverlayComposesWithTrustAnchors(t *testing.T) {
	p := GetStrict(overlayBase)
	if p == nil {
		t.Fatalf("preset %q is not registered", overlayBase)
	}
	spec, err := SpecForWithOverlay(p.ClientHelloID, 1, p.SignatureAlgorithms,
		[][]byte{{0x82, 0xdf, 0x13, 0x02, 0x01}},
		ExtensionOverlay{Add: []uint16{extSCT}, Drop: []uint16{extRecordSizeLimit}})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	hello := parseHello(t, helloBytes(t, spec))
	want := map[uint16]bool{extTrustAnchors: false, extSCT: false}
	for _, e := range hello.Extensions {
		if e.ID == extRecordSizeLimit {
			t.Error("record_size_limit survived the drop")
		}
		if _, ok := want[e.ID]; ok {
			want[e.ID] = true
		}
	}
	for id, present := range want {
		if !present {
			t.Errorf("extension %d (0x%04x) is not on the wire", id, id)
		}
	}
}

// A codepoint with no payload in the table is refused at load time rather than
// sent as an empty body, which is not the extension and would be on the wire
// for every connection the preset makes.
func TestExtensionOverlayRejectsUnsupportedAdditions(t *testing.T) {
	for name, overlay := range map[string]string{
		"no payload defined": `{"add":[17613]}`,
		"add and drop agree": `{"add":[28],"drop":[28]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadAndBuildPresetFromJSON([]byte(
				`{"version":1,"preset":{"name":"overlay-bad","based_on":"` + overlayBase + `",
				  "tls":{"extension_overlay":` + overlay + `}}}`))
			if err == nil {
				t.Fatal("the loader accepted an overlay it cannot put on the wire")
			}
		})
	}
	// Dropping a codepoint with no payload is fine: removing an extension needs
	// no payload, and refusing it would make the table govern the wrong half.
	if _, err := LoadAndBuildPresetFromJSON([]byte(
		`{"version":1,"preset":{"name":"overlay-drop-any","based_on":"` + overlayBase + `",
		  "tls":{"extension_overlay":{"drop":[17613]}}}}`)); err != nil {
		t.Fatalf("the loader refused a drop that needs no payload: %v", err)
	}
}

// An extension the handshake needs, or that every client sends for a reason,
// cannot be dropped. The add table cannot police this, because dropping needs
// no payload, and a hello missing one of these either fails to negotiate or
// announces itself as something no client is.
func TestExtensionOverlayRefusesUndroppableExtensions(t *testing.T) {
	for id := range undroppableExtensions {
		t.Run(fmt.Sprintf("drop_%d", id), func(t *testing.T) {
			err := ExtensionOverlay{Drop: []uint16{id}}.Validate()
			if err == nil {
				t.Fatalf("dropping extension %d (0x%04x) was accepted", id, id)
			}
			// The message has to say which codepoint and why, the way the add
			// check does; a bare rejection leaves the author guessing.
			for _, want := range []string{fmt.Sprintf("%d", id), fmt.Sprintf("0x%04x", id)} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %s", err, want)
				}
			}
			if len(err.Error()) < 40 {
				t.Errorf("error %q does not say why", err)
			}
		})
	}

	// The whole set at once is refused too, on the first one it meets.
	all := slices.Sorted(maps.Keys(undroppableExtensions))
	if err := (ExtensionOverlay{Drop: all}).Validate(); err == nil {
		t.Error("an overlay dropping every protected extension was accepted")
	}
}

// The extensions a candidate actually perturbs stay droppable. A guard that
// swept up the ordinary ones would leave the overlay with nothing to do.
func TestExtensionOverlayAllowsOrdinaryDrops(t *testing.T) {
	for _, id := range []uint16{
		extStatusRequest,   // 5
		extSCT,             // 18
		extPadding,         // 21
		extEMS,             // 23
		extCompressCert,    // 27
		extRecordSizeLimit, // 28
		extSessionTicket,   // 35
		extALPS,            // 17613
		extECH,             // 65037
		extRenegotiation,   // 65281
	} {
		t.Run(fmt.Sprintf("drop_%d", id), func(t *testing.T) {
			if err := (ExtensionOverlay{Drop: []uint16{id}}).Validate(); err != nil {
				t.Fatalf("dropping extension %d (0x%04x) was refused: %v", id, id, err)
			}
		})
	}
}

// The guard is a load-time check, so a preset naming one fails to build rather
// than failing on the wire.
func TestExtensionOverlayUndroppableFailsTheLoad(t *testing.T) {
	_, err := LoadAndBuildPresetFromJSON([]byte(
		`{"version":1,"preset":{"name":"overlay-nodrop","based_on":"` + overlayBase + `",
		  "tls":{"extension_overlay":{"drop":[13]}}}}`))
	if err == nil {
		t.Fatal("the loader accepted a preset that drops signature_algorithms")
	}
	if !strings.Contains(err.Error(), "extension_overlay") {
		t.Errorf("error %q does not say which block was rejected", err)
	}
}

// The JSON key round-trips through describe, so a preset built from a described
// one still carries the overlay.
func TestExtensionOverlayJSONRoundTrip(t *testing.T) {
	spec := `{"version":1,"preset":{"name":"overlay-rt","based_on":"` + overlayBase + `",
		"tls":{"extension_overlay":{"add":[5,18],"drop":[28]}}}}`

	p, err := LoadAndBuildPresetFromJSON([]byte(spec))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := p.ExtensionOverlay; len(got.Add) != 2 || len(got.Drop) != 1 {
		t.Fatalf("loaded overlay is %+v, want two additions and one drop", got)
	}
	if err := RegisterStrict(p.Name, p); err != nil {
		t.Fatalf("register: %v", err)
	}
	defer Unregister(p.Name)

	described, err := Describe(p.Name)
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if !strings.Contains(described, `"extension_overlay"`) {
		t.Fatal("describe dropped extension_overlay, so a round-trip loses it")
	}
	rt, err := LoadAndBuildPresetFromJSON([]byte(described))
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if fmt.Sprint(rt.ExtensionOverlay) != fmt.Sprint(p.ExtensionOverlay) {
		t.Errorf("round-trip left %+v, want %+v", rt.ExtensionOverlay, p.ExtensionOverlay)
	}

	// A preset that sets none does not advertise the key.
	base, err := Describe(overlayBase)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(base, `"extension_overlay"`) {
		t.Errorf("%s advertises extension_overlay; it sends the hello uTLS builds for it", overlayBase)
	}
}

// An overlay on one preset must not reach the preset it inherits from, which a
// shared backing array would allow.
func TestExtensionOverlayIsClonedFromTheBase(t *testing.T) {
	p, err := LoadAndBuildPresetFromJSON([]byte(
		`{"version":1,"preset":{"name":"overlay-clone","based_on":"` + overlayBase + `",
		  "tls":{"extension_overlay":{"add":[18]}}}}`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	p.ExtensionOverlay.Add[0] = extSessionTicket

	if base := GetStrict(overlayBase); !base.ExtensionOverlay.IsZero() {
		t.Fatalf("%s picked up an overlay from a preset based on it: %+v",
			overlayBase, base.ExtensionOverlay)
	}
}

// overlayPayloadSources names, for each extension the table can add, a preset
// that really sends it. The table's comments say where each payload comes from;
// this is that claim as a test.
var overlayPayloadSources = map[uint16]string{
	5:  "firefox-133-windows",
	18: "safari-17-ios",
	28: "firefox-133-windows",
	34: "firefox-133-windows",
	35: "firefox-133-windows",
}

// Every payload the overlay can add is the one the client it was taken from
// puts on the wire, compared as bytes rather than as a description.
//
// This is what stops the table drifting into codepoints with invented bodies.
// An extension added with the wrong payload still produces a working handshake,
// so nothing else here would notice, and it would be on every connection the
// preset makes.
func TestOverlayPayloadsMatchTheClientsTheyComeFrom(t *testing.T) {
	for id, build := range overlayExtensions {
		source, ok := overlayPayloadSources[id]
		if !ok {
			t.Errorf("extension %d (0x%04x) can be added but names no client to check it against", id, id)
			continue
		}
		t.Run(fmt.Sprintf("%d_from_%s", id, source), func(t *testing.T) {
			p := GetStrict(source)
			if p == nil {
				t.Fatalf("preset %q is not registered", source)
			}
			spec, err := SpecFor(p.ClientHelloID, 1, p.SignatureAlgorithms)
			if err != nil {
				t.Fatalf("build spec: %v", err)
			}
			var want []byte
			for _, e := range spec.Extensions {
				if got, ok := extensionID(e); ok && got == id {
					want = extensionWire(t, e)
				}
			}
			if want == nil {
				t.Fatalf("%s does not send extension %d (0x%04x), so it cannot be the source of its payload",
					source, id, id)
			}
			if got := extensionWire(t, build()); !bytes.Equal(got, want) {
				t.Errorf("the overlay sends % x, but %s sends % x", got, source, want)
			}
		})
	}
}

// extensionWire serializes one extension the way a hello carries it.
func extensionWire(t *testing.T, e tls.TLSExtension) []byte {
	t.Helper()
	buf := make([]byte, e.Len())
	if _, err := e.Read(buf); err != nil && err.Error() != "EOF" {
		t.Fatalf("serialize %T: %v", e, err)
	}
	return buf
}

// The overlay has to work on a Chrome hello too, which is the case it exists
// for. Chrome permutes its extensions per spec, so the list is compared as a
// set: every extension the base sends still goes out with the same bytes,
// minus the one dropped and plus the one added.
func TestOverlayOnAPermutingBase(t *testing.T) {
	const name = "chrome-151-windows"
	p := GetStrict(name)
	if p == nil {
		t.Fatalf("preset %q is not registered", name)
	}
	build := func(o ExtensionOverlay) map[uint16]string {
		spec, err := SpecForWithOverlay(p.ClientHelloID, 1, p.SignatureAlgorithms, p.TrustAnchors, o)
		if err != nil {
			t.Fatalf("build spec: %v", err)
		}
		out := map[uint16]string{}
		for _, e := range parseHello(t, helloBytes(t, spec)).Extensions {
			out[e.ID] = fmt.Sprintf("% x", e.Bytes)
		}
		return out
	}

	base := build(ExtensionOverlay{})
	over := build(ExtensionOverlay{Add: []uint16{extRecordSizeLimit}, Drop: []uint16{extSCT}})

	if _, ok := over[extRecordSizeLimit]; !ok {
		t.Error("record_size_limit is not on the wire after being added")
	}
	if _, ok := over[extSCT]; ok {
		t.Error("signed_certificate_timestamp survived the drop")
	}
	for id, want := range base {
		if id == extSCT {
			continue
		}
		got, ok := over[id]
		if !ok {
			t.Errorf("extension %d (0x%04x) disappeared, and the overlay did not name it", id, id)
			continue
		}
		// key_share, the GREASE ECH payload and the GREASE codepoints are all
		// redrawn per hello, so only their presence is comparable here; the
		// byte-level check on the non-permuting base covers the rest.
		if carriesFreshKeyMaterial(id) || isGREASECodepoint(id) {
			continue
		}
		if got != want {
			t.Errorf("extension %d (0x%04x) changed:\n got %s\nwant %s", id, id, got, want)
		}
	}
	if len(over) != len(base) {
		t.Errorf("the hello carries %d distinct extensions, want %d", len(over), len(base))
	}
}

func isGREASECodepoint(v uint16) bool {
	return v&0x0f0f == 0x0a0a && byte(v>>8) == byte(v)
}

// A resuming hello is the fresh one with pre_shared_key appended last, and an
// addition has to land ahead of that as well as ahead of the trailing GREASE.
// This is the shape the pooled path builds whenever a session resumes, so it is
// the one an overlay is most likely to get wrong: the extension uTLS appends is
// a UtlsPreSharedKeyExtension rather than the FakePreSharedKeyExtension the
// synthetic test uses, and only the interface they share keeps it pinned.
func TestApplyExtensionOverlayOnAResumingHello(t *testing.T) {
	const name = "chrome-151-windows"
	p := GetStrict(name)
	if p == nil {
		t.Fatalf("preset %q is not registered", name)
	}
	if p.PSKClientHelloID.Client == "" {
		t.Fatalf("%s has no resumption identity, so this test proves nothing", name)
	}

	fresh, err := SpecForWithOverlay(p.ClientHelloID, 1, p.SignatureAlgorithms, p.TrustAnchors, ExtensionOverlay{})
	if err != nil {
		t.Fatalf("fresh spec: %v", err)
	}
	psk, err := SpecForWithOverlay(p.PSKClientHelloID, 1, p.SignatureAlgorithms, p.TrustAnchors,
		ExtensionOverlay{Add: []uint16{extRecordSizeLimit}, Drop: []uint16{extSCT}})
	if err != nil {
		t.Fatalf("resuming spec: %v", err)
	}

	// The resuming hello carries one extension more than the fresh one before
	// the overlay touches it, and the overlay here is add-one drop-one.
	if got, want := len(psk.Extensions), len(fresh.Extensions)+1; got != want {
		t.Errorf("resuming hello has %d extensions, want %d", got, want)
	}

	last := psk.Extensions[len(psk.Extensions)-1]
	if _, ok := last.(tls.PreSharedKeyExtension); !ok {
		t.Fatalf("last extension is %T, want the pre_shared_key RFC 8446 4.2.11 requires there", last)
	}
	if _, ok := psk.Extensions[len(psk.Extensions)-2].(*tls.UtlsGREASEExtension); !ok {
		t.Errorf("the trailing GREASE extension moved: %T", psk.Extensions[len(psk.Extensions)-2])
	}
	at := -1
	for i, e := range psk.Extensions {
		if _, ok := e.(*tls.FakeRecordSizeLimitExtension); ok {
			at = i
		}
	}
	if want := len(psk.Extensions) - 3; at != want {
		t.Errorf("record_size_limit landed at index %d, want %d, ahead of the trailing GREASE and pre_shared_key", at, want)
	}
	for _, e := range psk.Extensions {
		if id, ok := extensionID(e); ok && id == extSCT {
			t.Error("signed_certificate_timestamp survived the drop on the resuming hello")
		}
	}
}
