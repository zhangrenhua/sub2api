// Package sensitiveword provides a byte-level Aho-Corasick multi-pattern
// matcher used to detect disallowed terms in gateway request payloads.
//
// Patterns and input text are both normalized via Normalize before matching:
// ASCII letters are folded to lowercase and zero-width characters
// (U+200B/200C/200D/FEFF) are removed. Byte-level matching is safe for UTF-8
// Chinese patterns because continuation bytes (0x80-0xBF) never collide with
// lead bytes, so no cross-character false matches can occur for well-formed
// inputs.
package sensitiveword

import "strings"

// Matcher holds a compiled Aho-Corasick automaton. The zero value is not usable;
// always construct via NewMatcher. A nil *Matcher is safe and always returns
// no match from FirstMatch.
type Matcher struct {
	nodes []acNode
}

type acNode struct {
	children [256]int32
	fail     int32
	// output is the pattern that terminates at this node (after propagation
	// along failure links). Empty when the node is not an accept state.
	output string
}

// NewMatcher compiles the given patterns into an AC automaton. Empty patterns
// are skipped. Patterns are normalized before insertion, so callers may pass
// raw words. Returns nil when no usable pattern remains.
func NewMatcher(patterns []string) *Matcher {
	cleaned := make([]string, 0, len(patterns))
	for _, p := range patterns {
		p = Normalize(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	if len(cleaned) == 0 {
		return nil
	}

	m := &Matcher{nodes: make([]acNode, 1)}
	for i := range m.nodes[0].children {
		m.nodes[0].children[i] = -1
	}

	// Build trie.
	for _, p := range cleaned {
		cur := int32(0)
		for i := 0; i < len(p); i++ {
			b := p[i]
			next := m.nodes[cur].children[b]
			if next == -1 {
				var node acNode
				for j := range node.children {
					node.children[j] = -1
				}
				m.nodes = append(m.nodes, node)
				next = int32(len(m.nodes) - 1)
				m.nodes[cur].children[b] = next
			}
			cur = next
		}
		if m.nodes[cur].output == "" {
			m.nodes[cur].output = p
		}
	}

	// BFS to build failure links and merge goto transitions.
	queue := make([]int32, 0, len(m.nodes))
	for b := 0; b < 256; b++ {
		ch := m.nodes[0].children[b]
		if ch == -1 {
			m.nodes[0].children[b] = 0
		} else {
			m.nodes[ch].fail = 0
			queue = append(queue, ch)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for b := 0; b < 256; b++ {
			v := m.nodes[u].children[b]
			fu := m.nodes[u].fail
			if v == -1 {
				m.nodes[u].children[b] = m.nodes[fu].children[b]
				continue
			}
			m.nodes[v].fail = m.nodes[fu].children[b]
			if m.nodes[v].output == "" {
				if suffixOut := m.nodes[m.nodes[v].fail].output; suffixOut != "" {
					m.nodes[v].output = suffixOut
				}
			}
			queue = append(queue, v)
		}
	}
	return m
}

// FirstMatch scans s and returns the first matched pattern.
// s must already be normalized (see Normalize); callers typically normalize
// once per text field before calling.
func (m *Matcher) FirstMatch(s string) (string, bool) {
	if m == nil || len(m.nodes) == 0 {
		return "", false
	}
	cur := int32(0)
	for i := 0; i < len(s); i++ {
		cur = m.nodes[cur].children[s[i]]
		if out := m.nodes[cur].output; out != "" {
			return out, true
		}
	}
	return "", false
}

// Normalize folds ASCII to lowercase and strips zero-width characters
// (U+200B/200C/200D/FEFF). Non-ASCII bytes are preserved verbatim.
func Normalize(s string) string {
	needsCopy := false
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 'A' && b <= 'Z' {
			needsCopy = true
			break
		}
		if b == 0xE2 || b == 0xEF {
			// Potential start of a zero-width sequence; inspect.
			needsCopy = true
			break
		}
	}
	if !needsCopy {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+2 < len(s) {
			b0, b1, b2 := s[i], s[i+1], s[i+2]
			// U+200B/200C/200D → E2 80 8B/8C/8D
			if b0 == 0xE2 && b1 == 0x80 && (b2 == 0x8B || b2 == 0x8C || b2 == 0x8D) {
				i += 3
				continue
			}
			// U+FEFF → EF BB BF
			if b0 == 0xEF && b1 == 0xBB && b2 == 0xBF {
				i += 3
				continue
			}
		}
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		sb.WriteByte(c)
		i++
	}
	return sb.String()
}
