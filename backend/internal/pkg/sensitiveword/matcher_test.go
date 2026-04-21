package sensitiveword

import "testing"

func TestNewMatcherNil(t *testing.T) {
	if m := NewMatcher(nil); m != nil {
		t.Fatalf("expected nil for empty patterns, got %#v", m)
	}
	if m := NewMatcher([]string{"", "   "}); m != nil {
		t.Fatalf("expected nil when only blank patterns, got %#v", m)
	}
	// FirstMatch on nil must not panic.
	if _, ok := (*Matcher)(nil).FirstMatch("anything"); ok {
		t.Fatalf("nil matcher must never match")
	}
}

func TestFirstMatchASCII(t *testing.T) {
	m := NewMatcher([]string{"CSAM", "child porn"})
	cases := []struct {
		in      string
		want    string
		wantHit bool
	}{
		{Normalize("this has csam somewhere"), "csam", true},
		{Normalize("THIS HAS CSAM UPPERCASE"), "csam", true},
		{Normalize("talks about child porn here"), "child porn", true},
		{Normalize("completely benign content"), "", false},
	}
	for _, tc := range cases {
		got, ok := m.FirstMatch(tc.in)
		if ok != tc.wantHit || got != tc.want {
			t.Errorf("FirstMatch(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantHit)
		}
	}
}

func TestFirstMatchChinese(t *testing.T) {
	m := NewMatcher([]string{"奶子", "炸弹制作"})
	if w, ok := m.FirstMatch(Normalize("前面是奶子后面是别的")); !ok || w != "奶子" {
		t.Errorf("want 奶子, got (%q, %v)", w, ok)
	}
	if w, ok := m.FirstMatch(Normalize("讨论炸弹制作方法")); !ok || w != "炸弹制作" {
		t.Errorf("want 炸弹制作, got (%q, %v)", w, ok)
	}
	if _, ok := m.FirstMatch(Normalize("完全正常的中文内容")); ok {
		t.Errorf("unexpected hit on benign text")
	}
}

func TestNormalizeStripsZeroWidth(t *testing.T) {
	// U+200B ZWSP inserted between characters.
	in := "C​SAM"
	got := Normalize(in)
	if got != "csam" {
		t.Fatalf("Normalize(%q) = %q, want %q", in, got, "csam")
	}
	// U+FEFF BOM prefix (written as explicit bytes to avoid BOM in source).
	in2 := "\xEF\xBB\xBFHello"
	got2 := Normalize(in2)
	if got2 != "hello" {
		t.Fatalf("Normalize(%q) = %q, want %q", in2, got2, "hello")
	}
}

func TestNormalizePreservesNonASCII(t *testing.T) {
	in := "奶子"
	if Normalize(in) != in {
		t.Fatalf("Normalize must preserve CJK bytes")
	}
}

func TestFirstMatchMatchesAcrossZeroWidth(t *testing.T) {
	m := NewMatcher([]string{"CSAM"})
	// After normalization zero-width chars are stripped so the pattern is
	// contiguous in the scanned input.
	text := Normalize("C​S‌A‍M")
	if w, ok := m.FirstMatch(text); !ok || w != "csam" {
		t.Fatalf("want csam after ZW strip, got (%q, %v)", w, ok)
	}
}

func TestMultiplePatternsOverlap(t *testing.T) {
	// "he", "she", "hers", "his" is the classic AC example set.
	m := NewMatcher([]string{"he", "she", "hers", "his"})
	cases := []struct {
		in   string
		want string
	}{
		{"ushers", "she"}, // "she" ends at index 3
		{"his", "his"},
		{"ahem", "he"},
	}
	for _, tc := range cases {
		w, ok := m.FirstMatch(tc.in)
		if !ok || w != tc.want {
			t.Errorf("FirstMatch(%q) = (%q, %v), want %q", tc.in, w, ok, tc.want)
		}
	}
}
