package handlers

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/net/idna"
	"golang.org/x/text/unicode/rangetable"

	"github.com/marcinskalski/sailor-buddy/backend/internal/types"
)

// groupChatURL validates an optional group chat link. A blank link clears it;
// anything other than an absolute http or https URL is a client error (422).
func groupChatURL(s *string) (types.NullString, error) {
	v := nullTrimmed(s)
	if !v.Valid {
		return v, nil
	}
	u, err := url.Parse(v.String)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || !validPort(u.Port()) ||
		failsDomainToASCII(u.Hostname()) || invalidNumericHost(browserHost(u.Hostname())) {
		return types.NullString{}, huma.Error422UnprocessableEntity("group_chat_url must be an http or https URL")
	}
	return v, nil
}

// validPort accepts an absent port or one in 1-65535; url.Parse does not
// check the range, while browsers reject such links.
func validPort(p string) bool {
	if p == "" {
		return true
	}
	n, err := strconv.Atoi(p)
	return err == nil && n >= 1 && n <= 65535
}

// whatwgIDNA approximates the WHATWG "domain to ASCII" profile: UTS46
// mapping without STD3, hyphen or DNS-length checks.
var whatwgIDNA = idna.New(
	idna.MapForLookup(),
	idna.Transitional(false),
	idna.StrictDomainName(false),
	idna.CheckHyphens(false),
	idna.VerifyDNSLength(false),
)

// browserHost maps host the way a browser does before its IPv4 check, so
// full-width digits or U+3002 dots read as ASCII. IDNA errors are not
// grounds for rejection here: idna may flag labels a browser accepts (e.g.
// newer Unicode), so its best-effort mapping is still used, and the raw
// host only when it yields nothing.
func browserHost(host string) string {
	mapped, _ := whatwgIDNA.ToASCII(host)
	if mapped == "" {
		return host
	}
	return mapped
}

// joinerIDNA runs only the UTS46 CheckJoiners (CONTEXTJ) rules: no mapping,
// no code point or bidi validation.
var joinerIDNA = idna.New(idna.CheckJoiners(true))

// assignedByUnicode13 holds the code points assigned by Unicode 13. A label
// is rejected for starting with a combining mark only when the mark is in
// this set: parsers with older mark tables (Node's new URL() among them)
// accept labels that start with a newer mark such as U+11F00.
var assignedByUnicode13 = rangetable.Assigned("13.0.0")

// failsDomainToASCII reports whether the WHATWG URL parser (used by browsers
// and the frontend's new URL()) rejects host in its "domain to ASCII" step,
// which url.Parse does not run. It checks each UTS46-mapped label for an
// xn-- label that does not decode to valid non-ASCII text, an unassigned or
// private-use code point or U+FFFD, a leading combining mark, and ZWNJ/ZWJ outside their
// CONTEXTJ contexts. An IPv6 literal with a zone ID fails too: url.Parse
// accepts "[fe80::1%25eth0]", browsers do not.
//
// Only these checks reject. A bare idna error is not enough: idna also flags
// labels a browser accepts.
func failsDomainToASCII(host string) bool {
	if strings.Contains(host, ":") {
		return strings.Contains(host, "%")
	}
	for raw := range strings.FieldsFuncSeq(host, isLabelSeparator) {
		// ToUnicode maps the label and decodes it if it is a valid xn--
		// label, returning that result even alongside an error.
		label, _ := whatwgIDNA.ToUnicode(raw)
		if isACELabel(raw) && !validDecodedACE(label) {
			// The WHATWG parser rejects it; Node's new URL() skips IDNA for
			// all-ASCII hosts and accepts it.
			return true
		}
		for i, r := range label {
			if r == unicode.ReplacementChar || unicode.In(r, unicode.Co, unicode.Cn) {
				return true
			}
			if i == 0 && unicode.Is(unicode.M, r) && unicode.Is(assignedByUnicode13, r) {
				return true
			}
		}
		// The leading "a" keeps idna's own leading-mark check out of this
		// one; a non-joining letter does not change any CONTEXTJ result.
		if strings.ContainsAny(label, "\u200c\u200d") {
			if _, err := joinerIDNA.ToUnicode("a" + label); err != nil {
				return true
			}
		}
	}
	return false
}

// isLabelSeparator reports the code points UTS46 maps to a label dot.
func isLabelSeparator(r rune) bool {
	return r == '.' || r == '\u3002' || r == '\uff0e' || r == '\uff61'
}

// isACELabel reports whether raw, once UTS46-mapped, starts with "xn--"
// (e.g. full-width "ｘｎ--"). The leading "a" stops ToUnicode decoding it.
func isACELabel(raw string) bool {
	mapped, _ := whatwgIDNA.ToUnicode("a" + raw)
	return strings.HasPrefix(mapped, "axn--")
}

// validDecodedACE reports whether label, the decoded form of an xn--
// label, is one UTS46 accepts: punycode that decoded (an undecodable label,
// such as "xn--zz" or "xn--é", stays "xn--..."), to non-ASCII text that
// UTS46 mapping leaves unchanged (no uppercase Ż from "xn--gha").
func validDecodedACE(label string) bool {
	if label == "" || isASCII(label) || strings.HasPrefix(label, "xn--") {
		return false
	}
	remapped, _ := whatwgIDNA.ToUnicode(label)
	return remapped == label
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// invalidNumericHost reports whether host is one the WHATWG URL parser
// (used by browsers and the frontend's new URL()) rejects because it ends in
// a number but is not a valid IPv4 address, e.g. "192.168.1.999",
// "1.2.3.4.5" or "chat.123". url.Parse accepts such hosts. IPv6 literals
// and non-numeric domains pass through unchanged.
func invalidNumericHost(host string) bool {
	if strings.Contains(host, ":") {
		return false
	}
	parts := strings.Split(host, ".")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if !endsInNumber(parts[len(parts)-1]) {
		return false
	}
	return !validIPv4(parts)
}

// endsInNumber mirrors the WHATWG "ends in a number" checker for the last
// host label: all decimal digits, or a 0x-prefixed hex number.
func endsInNumber(last string) bool {
	if last == "" {
		return false
	}
	if allDigits(last) {
		return true
	}
	_, ok := ipv4Number(last)
	return ok
}

// validIPv4 mirrors the WHATWG IPv4 parser over host labels with any
// trailing empty label already removed.
func validIPv4(parts []string) bool {
	if len(parts) > 4 {
		return false
	}
	nums := make([]uint64, len(parts))
	for i, p := range parts {
		n, ok := ipv4Number(p)
		if !ok {
			return false
		}
		if i < len(parts)-1 && n > 255 {
			return false
		}
		nums[i] = n
	}
	return nums[len(nums)-1] < 1<<(8*(5-len(parts)))
}

// ipv4Number parses one IPv4 label the WHATWG way: 0x/0X prefix is hex, a
// leading 0 is octal, otherwise decimal. A bare prefix ("0x") is zero.
func ipv4Number(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	base := 10
	switch {
	case len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X"):
		base, s = 16, s[2:]
	case len(s) >= 2 && s[0] == '0':
		base, s = 8, s[1:]
	}
	if s == "" {
		return 0, true
	}
	for _, r := range s {
		if d := digitValue(r); d < 0 || d >= base {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		// Every digit is valid, so the only error is overflow: still a
		// number, and one that fails the range check.
		return 1 << 63, true
	}
	return n, true
}

// digitValue returns the value of an ASCII hex digit, or -1.
func digitValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	}
	return -1
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
