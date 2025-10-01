package database

import (
	"strings"
	"unicode"

	sha "next.orly.dev/pkg/crypto/sha256"
)

// TokenHashes extracts unique word hashes (8-byte truncated sha256) from content.
// Rules:
// - Unicode-aware: words are sequences of letters or numbers.
// - Lowercased using unicode case mapping.
// - Ignore URLs (starting with http://, https://, www., or containing "://").
// - Ignore nostr: URIs and #[n] mentions.
// - Ignore words shorter than 2 runes.
// - Exclude 64-character hexadecimal strings (likely IDs/pubkeys).
func TokenHashes(content []byte) [][]byte {
	s := string(content)
	var out [][]byte
	seen := make(map[string]struct{})

	i := 0
	for i < len(s) {
		r, size := rune(s[i]), 1
		if r >= 0x80 {
			r, size = utf8DecodeRuneInString(s[i:])
		}

		// Skip whitespace
		if unicode.IsSpace(r) {
			i += size
			continue
		}

		// Skip URLs and schemes
		if hasPrefixFold(s[i:], "http://") || hasPrefixFold(s[i:], "https://") || hasPrefixFold(s[i:], "nostr:") || hasPrefixFold(s[i:], "www.") {
			i = skipUntilSpace(s, i)
			continue
		}
		// If token contains "://" ahead, treat as URL and skip to space
		if j := strings.Index(s[i:], "://"); j == 0 || (j > 0 && isWordStart(r)) {
			// Only if it's at start of token
			before := s[i : i+j]
			if len(before) == 0 || allAlphaNum(before) {
				i = skipUntilSpace(s, i)
				continue
			}
		}
		// Skip #[n] mentions
		if r == '#' && i+size < len(s) && s[i+size] == '[' {
			end := strings.IndexByte(s[i:], ']')
			if end >= 0 {
				i += end + 1
				continue
			}
		}

		// Collect a word
		start := i
		var runes []rune
		for i < len(s) {
			r2, size2 := rune(s[i]), 1
			if r2 >= 0x80 {
				r2, size2 = utf8DecodeRuneInString(s[i:])
			}
			if unicode.IsLetter(r2) || unicode.IsNumber(r2) {
				runes = append(runes, unicode.ToLower(r2))
				i += size2
				continue
			}
			break
		}
		_ = start
		if len(runes) >= 2 {
			w := string(runes)
			// Exclude 64-char hex strings
			if isHex64(w) {
				continue
			}
			if _, ok := seen[w]; !ok {
				seen[w] = struct{}{}
				h := sha.Sum256([]byte(w))
				out = append(out, h[:8])
			}
		}
	}
	return out
}

func hasPrefixFold(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := s[i]
		p := prefix[i]
		if c == p {
			continue
		}
		// ASCII case-insensitive
		if 'A' <= c && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		if 'A' <= p && p <= 'Z' {
			p = p - 'A' + 'a'
		}
		if c != p {
			return false
		}
	}
	return true
}

func skipUntilSpace(s string, i int) int {
	for i < len(s) {
		r, size := rune(s[i]), 1
		if r >= 0x80 {
			r, size = utf8DecodeRuneInString(s[i:])
		}
		if unicode.IsSpace(r) {
			return i
		}
		i += size
	}
	return i
}

func allAlphaNum(s string) bool {
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r)) {
			return false
		}
	}
	return true
}

func isWordStart(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// Minimal utf8 rune decode without importing utf8 to avoid extra deps elsewhere
func utf8DecodeRuneInString(s string) (r rune, size int) {
	// Fallback to standard library if available; however, using basic decoding
	for i := 1; i <= 4 && i <= len(s); i++ {
		r, size = rune(s[0]), 1
		if r < 0x80 {
			return r, 1
		}
		// Use stdlib for correctness
		return []rune(s[:i])[0], len(string([]rune(s[:i])[0]))
	}
	return rune(s[0]), 1
}

// isHex64 returns true if s is exactly 64 hex characters (0-9, a-f)
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < 64; i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			continue
		}
		if c >= 'A' && c <= 'F' {
			continue
		}
		return false
	}
	return true
}
