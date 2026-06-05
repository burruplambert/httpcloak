package fingerprint

import (
	"regexp"
	"strings"
	"testing"
)

// brandEntry is one "Brand";v="version" pair from a sec-ch-ua style list.
type brandEntry struct {
	brand string
	ver   string
}

// parseBrandList parses a sec-ch-ua / full-version-list value into ordered
// (brand, version) pairs. Independent of the production parser so the test
// actually checks the output rather than re-deriving it.
func parseBrandList(s string) []brandEntry {
	var out []brandEntry
	depth := 0
	start := 0
	flush := func(seg string) {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			return
		}
		// brand = text inside the first quote pair
		first := strings.IndexByte(seg, '"')
		second := strings.IndexByte(seg[first+1:], '"')
		brand := seg[first+1 : first+1+second]
		ver := ""
		if i := strings.Index(seg, `v="`); i != -1 {
			rest := seg[i+3:]
			if j := strings.IndexByte(rest, '"'); j != -1 {
				ver = rest[:j]
			}
		}
		out = append(out, brandEntry{brand: brand, ver: ver})
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			depth ^= 1
		case ',':
			if depth == 0 {
				flush(s[start:i])
				start = i + 1
			}
		}
	}
	flush(s[start:])
	return out
}

var chromeUAMajorRe = regexp.MustCompile(`Chrome/(\d+)`)

// TestClientHintCoherence locks the core property of the fix: for every Chrome
// preset, the resolved sec-ch-ua-full-version-list must agree with the sec-ch-ua
// trio (identical brand names, order and GREASE token; each major version equal)
// and with the User-Agent's major. This is what catches drift when chrome-150
// gets added: a new preset whose hints don't line up fails here.
func TestClientHintCoherence(t *testing.T) {
	names := Available()
	checked := 0
	for _, name := range names {
		p := Get(name)
		if p == nil {
			continue
		}
		secChUa := p.Headers["sec-ch-ua"]
		if secChUa == "" {
			continue // non-Chrome preset (no UA-CH)
		}
		checked++
		t.Run(name, func(t *testing.T) {
			ch := p.ResolveClientHints()

			sec := parseBrandList(secChUa)
			fvl := parseBrandList(ch.UAFullVersionList)

			if len(sec) != len(fvl) {
				t.Fatalf("brand count mismatch: sec-ch-ua has %d, full-version-list has %d\n  sec=%s\n  fvl=%s",
					len(sec), len(fvl), secChUa, ch.UAFullVersionList)
			}
			for i := range sec {
				// Same brand name, same position, same GREASE token.
				if sec[i].brand != fvl[i].brand {
					t.Errorf("brand[%d] mismatch: sec-ch-ua=%q full-version-list=%q (order/GREASE drift)", i, sec[i].brand, fvl[i].brand)
				}
				// full-version-list major must equal the sec-ch-ua version.
				major := fvl[i].ver
				if dot := strings.IndexByte(major, '.'); dot != -1 {
					major = major[:dot]
				}
				if major != sec[i].ver {
					t.Errorf("brand %q version major mismatch: sec-ch-ua v=%q, full-version-list v=%q", sec[i].brand, sec[i].ver, fvl[i].ver)
				}
			}

			// The Google Chrome brand's major must match the UA's Chrome/NNN.
			if m := chromeUAMajorRe.FindStringSubmatch(p.UserAgent); len(m) == 2 {
				uaMajor := m[1]
				for _, e := range sec {
					if e.brand == "Google Chrome" && e.ver != uaMajor {
						t.Errorf("sec-ch-ua Chrome major %q != User-Agent Chrome major %q", e.ver, uaMajor)
					}
				}
			}

			// Linux sends an empty (quoted) platform version.
			if p.Headers["sec-ch-ua-platform"] == `"Linux"` && ch.UAPlatformVersion != `""` {
				t.Errorf("Linux sec-ch-ua-platform-version should be %q, got %q", `""`, ch.UAPlatformVersion)
			}
		})
	}
	if checked == 0 {
		t.Fatal("no Chrome presets exercised")
	}
}
