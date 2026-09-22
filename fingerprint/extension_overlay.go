package fingerprint

import (
	"fmt"
	"slices"

	tls "github.com/sardanioss/utls"
)

// ExtensionOverlay adds and removes extensions on the ClientHello a
// ClientHelloID produces, and changes nothing else about it.
//
// It generalises the trust_anchors list, which could only ever insert one
// specific extension. A preset that wants to describe a client which sends one
// extension more or one less than the hello uTLS ships for it had no way to say
// so short of abandoning the ClientHelloID for a JA3 or a captured hello, and
// both of those replace the whole ClientHello: a JA3 cannot express GREASE,
// ALPS or the key-share groups, and a capture freezes the values the client
// varies per handshake. The overlay keeps the hello and edits its extension
// list, so everything the ClientHelloID gets right stays right.
//
// Add and Drop are sets, not sequences. An extension the hello already carries
// is left exactly as the hello has it rather than replaced with the overlay's
// copy, because the hello's own is the more faithful of the two; an extension
// the hello does not carry is simply not dropped. Both cases are no-ops, so an
// overlay cannot make a hello worse by naming something already true of it.
//
// Only the extensions in overlayExtensions may be added, and only extensions
// outside undroppableExtensions may be dropped; Validate refuses the rest at
// load time. See those two tables for why a codepoint on its own is not enough
// in either direction.
type ExtensionOverlay struct {
	// Add lists the extensions to insert, by IANA codepoint.
	Add []uint16 `json:"add,omitempty"`
	// Drop lists the extensions to remove, by IANA codepoint.
	Drop []uint16 `json:"drop,omitempty"`
}

// IsZero reports whether the overlay would change nothing.
func (o ExtensionOverlay) IsZero() bool {
	return len(o.Add) == 0 && len(o.Drop) == 0
}

// Clone returns a copy that shares no backing array with o.
func (o ExtensionOverlay) Clone() ExtensionOverlay {
	return ExtensionOverlay{Add: slices.Clone(o.Add), Drop: slices.Clone(o.Drop)}
}

// Validate reports whether the overlay can be put on the wire.
//
// It runs at load time, not at handshake time, for the reason the JA3 and
// trust-anchor validation gives: a preset that cannot be expressed should fail
// where the mistake is, not on a connection that then succeeds while sending
// something nobody asked for. Once a Preset holds an overlay it is known good,
// which is what lets ApplyExtensionOverlay be infallible.
func (o ExtensionOverlay) Validate() error {
	for _, id := range o.Add {
		if _, ok := overlayExtensions[id]; !ok {
			return fmt.Errorf("cannot add extension %d (0x%04x): no payload is defined for it, "+
				"only %s may be added", id, id, formatCodepoints(SupportedOverlayExtensions()))
		}
	}
	for _, id := range o.Drop {
		if why, ok := undroppableExtensions[id]; ok {
			return fmt.Errorf("cannot drop extension %d (0x%04x): %s", id, id, why)
		}
	}
	for _, id := range o.Add {
		if slices.Contains(o.Drop, id) {
			return fmt.Errorf("extension %d (0x%04x) is in both add and drop", id, id)
		}
	}
	return nil
}

// undroppableExtensions are the codepoints an overlay may not remove, and the
// reason each one is on the list.
//
// Dropping needs no payload, so the add table cannot police it, and an overlay
// that removes the wrong extension produces a hello that either fails to
// negotiate or announces itself as something no client is. Both are worse than
// the shared fingerprint the overlay was reached for, and both are invisible
// from the preset: the handshake either quietly falls back or quietly succeeds
// while looking wrong.
//
// The list is the extensions whose removal breaks the handshake, plus ALPN.
// Everything else stays droppable, including extensions every preset in this
// library happens to send: ec_point_formats, extended_master_secret and
// renegotiation_info are all universal here, but TLS 1.3 ignores all three, a
// client that signals renegotiation support through the SCSV cipher omits
// renegotiation_info entirely, and a hello without them still works. Refusing
// those would trade a real capability for a judgement about what is plausible,
// which is what measurement is for.
//
// ALPN is the one entry that is not purely about the handshake completing. It
// is in because its absence is both a functional break and an unmistakable
// tell: an HTTP library whose hello offers no protocol list negotiates HTTP/1.1
// however the session is configured, and JA4 folds the first ALPN value into
// its first component, where no offer at all renders as "00" and no browser
// produces that.
var undroppableExtensions = map[uint16]string{
	0:  "server_name carries the host the handshake is for, and every preset here sends it",
	10: "supported_groups names the groups key_share draws from, so a hello without it has no group the peer can agree to",
	13: "signature_algorithms is how a TLS 1.3 client says which certificate signatures it accepts, and it is the part of the hello most closely inspected",
	16: "application_layer_protocol_negotiation is what negotiates HTTP/2, so a hello without it speaks HTTP/1.1 whatever the session asked for",
	41: "pre_shared_key is the resumption offer itself, and RFC 8446 4.2.11 requires it last rather than absent",
	43: "supported_versions is the only place a TLS 1.3 client says so, so a hello without it is a TLS 1.2 hello",
	45: "psk_key_exchange_modes is required alongside a resumption offer, and every preset here sends it",
	51: "key_share carries the client's key exchange, so a hello without it costs a HelloRetryRequest on every connection",
}

// overlayExtensions is the set an overlay may add, and the payload each one
// carries.
//
// A codepoint on its own is not enough to add an extension. record_size_limit
// with an empty body is not record_size_limit, it is a malformed copy of it,
// and unlike a mistake in a request it goes out on every connection the preset
// makes and is read by the peer before any application data exists. So every
// payload below is the one a client this library already ships puts on the
// wire, and the comment on each says which client that is. The values were read
// back out of those presets' own hellos rather than transcribed, so refreshing
// a preset moves them.
//
// The table is closed on purpose. An open payload API would let a caller put
// arbitrary bytes under an arbitrary codepoint, which is not a fingerprint any
// client has and is not something this library should help construct.
//
// trust_anchors (0xCA34) is deliberately absent: it has its own preset field,
// which carries the identifier list an empty payload could not, and two ways to
// send one extension is one too many.
var overlayExtensions = map[uint16]func() tls.TLSExtension{
	// status_request. Chrome, Firefox and Safari all send the same nine bytes:
	// OCSP, an empty responder ID list and no request extensions.
	5: func() tls.TLSExtension { return &tls.StatusRequestExtension{} },

	// signed_certificate_timestamp, empty, as Chrome and Safari send it.
	18: func() tls.TLSExtension { return &tls.SCTExtension{} },

	// record_size_limit. Firefox sends 0x4001, one byte over the 16384-byte
	// plaintext limit, which is what RFC 8449 2 asks a TLS 1.3 client to send
	// so the extra byte of the inner content type fits.
	28: func() tls.TLSExtension { return &tls.FakeRecordSizeLimitExtension{Limit: 0x4001} },

	// delegated_credentials, with the four algorithms Firefox offers.
	34: func() tls.TLSExtension {
		return &tls.FakeDelegatedCredentialsExtension{
			SupportedSignatureAlgorithms: []tls.SignatureScheme{
				tls.ECDSAWithP256AndSHA256,
				tls.ECDSAWithP384AndSHA384,
				tls.ECDSAWithP521AndSHA512,
				tls.ECDSAWithSHA1,
			},
		}
	},

	// session_ticket, empty, as Chrome and Firefox send it on a fresh
	// connection. A resuming hello fills it in, which is the transport's job
	// and not something an overlay should pre-empt.
	35: func() tls.TLSExtension { return &tls.SessionTicketExtension{} },
}

// SupportedOverlayExtensions returns the codepoints an overlay may add, in
// ascending order. It backs the load-time error message, so a rejected preset
// says what it could have asked for instead.
func SupportedOverlayExtensions() []uint16 {
	out := make([]uint16, 0, len(overlayExtensions))
	for id := range overlayExtensions {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// formatCodepoints renders a codepoint list for an error message.
func formatCodepoints(ids []uint16) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d (0x%04x)", id, id)
	}
	switch len(parts) {
	case 0:
		return "none"
	case 1:
		return parts[0]
	}
	return fmt.Sprintf("%s and %s", joinComma(parts[:len(parts)-1]), parts[len(parts)-1])
}

func joinComma(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}

// ApplyExtensionOverlay drops and adds extensions on a spec built from a
// ClientHelloID, and is a no-op for an overlay that names nothing.
//
// Drops happen first, so an overlay that removes an extension and adds another
// does not have its insert position decided by an extension on its way out.
// Additions land ahead of the trailing run that may not move, which is where
// ApplyTrustAnchors puts its extension and for the same reasons: GREASE
// brackets the list, padding sizes the record, and RFC 8446 4.2.11 requires
// pre_shared_key to be last. An extension appended after those would be pinned
// to the final slot, where it could not take part in the ordinary permutation
// the way a real client's does.
//
// It takes a pointer to the slice because adding and removing change the slice
// header rather than an element, and so that the HTTP/1.1 path can pass
// &tlsConn.Extensions where there is no spec.
//
// An unsupported codepoint in Add is skipped rather than reported: Validate
// rejects one at load time, so a Preset cannot carry one, and a hand-built
// Preset that does has already bypassed the check this would repeat.
func ApplyExtensionOverlay(exts *[]tls.TLSExtension, o ExtensionOverlay) {
	if exts == nil || o.IsZero() {
		return
	}

	if len(o.Drop) > 0 {
		*exts = slices.DeleteFunc(*exts, func(e tls.TLSExtension) bool {
			id, ok := extensionID(e)
			return ok && slices.Contains(o.Drop, id)
		})
	}

	var add []tls.TLSExtension
	for _, id := range o.Add {
		build, ok := overlayExtensions[id]
		if !ok || hasExtension(*exts, id) {
			continue
		}
		add = append(add, build())
	}
	if len(add) == 0 {
		return
	}

	at := pinnedTailStart(*exts)
	out := make([]tls.TLSExtension, 0, len(*exts)+len(add))
	out = append(out, (*exts)[:at]...)
	out = append(out, add...)
	out = append(out, (*exts)[at:]...)
	*exts = out
}

// pinnedTailStart returns the index at which the trailing run of extensions
// that may not move begins, which is where a new extension is inserted.
//
// GREASE brackets the list, padding sizes the record and RFC 8446 4.2.11 puts
// pre_shared_key last, so anything placed after those is pinned to the final
// slot instead of taking part in the ordinary extension permutation.
func pinnedTailStart(exts []tls.TLSExtension) int {
	at := len(exts)
	for at > 0 {
		switch exts[at-1].(type) {
		case *tls.UtlsGREASEExtension, *tls.UtlsPaddingExtension, tls.PreSharedKeyExtension:
			at--
			continue
		}
		break
	}
	return at
}

// hasExtension reports whether a spec already carries a codepoint.
func hasExtension(exts []tls.TLSExtension, id uint16) bool {
	return slices.ContainsFunc(exts, func(e tls.TLSExtension) bool {
		got, ok := extensionID(e)
		return ok && got == id
	})
}

// extensionID reports the codepoint an extension writes on the wire.
//
// Most answer through Read, which emits the two-byte codepoint ahead of the
// body. Three cannot, because before a handshake there is nothing for them to
// serialize: server_name has no host yet and padding has no hello to size
// itself against, both of which have a fixed codepoint and are answered
// directly, while a GREASE extension has no value assigned and reports none at
// all. That last one is deliberate rather than a gap: a GREASE codepoint is
// drawn per handshake, so naming one in an overlay could only ever match by
// accident, and matching it would let a drop remove the bracket a Chrome hello
// is recognised by.
func extensionID(ext tls.TLSExtension) (uint16, bool) {
	switch ext.(type) {
	case *tls.SNIExtension:
		return 0, true
	case *tls.UtlsPaddingExtension:
		return 21, true
	case *tls.UtlsGREASEExtension:
		return 0, false
	}
	// The error is ignored when the header came through: these Read
	// implementations fill the buffer and return io.EOF in the same call, the
	// way a bytes.Reader does, so a non-nil error says nothing about whether
	// the two bytes that matter were written.
	buf := make([]byte, ext.Len())
	if n, _ := ext.Read(buf); n < 2 {
		return 0, false
	}
	return uint16(buf[0])<<8 | uint16(buf[1]), true
}
