package updatecheck

import "testing"

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		current, latest string
		want            Severity
	}{
		{"0.5.0", "0.6.0", SeverityMinor},
		{"0.5.0", "1.0.0", SeverityMajor},
		{"0.5.0", "0.5.1", SeverityPatch},
		{"0.5.0", "0.5.0", SeverityNone},
		{"0.5.0", "0.4.99", SeverityNone},
		{"v0.5.0", "v0.6.0", SeverityMinor},
		{"abc", "0.6.0", SeverityNone},
		{"0.5.0", "abc", SeverityNone},
	}
	for _, tc := range cases {
		t.Run(tc.current+"->"+tc.latest, func(t *testing.T) {
			if got := CompareSemver(tc.current, tc.latest); got != tc.want {
				t.Errorf("CompareSemver(%q,%q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestParseSemver_EdgeCases(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", false},
		{"0.5", false},       // 2 parts
		{"0.5.0.0", false},   // 4 parts
		{"-1.2.3", false},    // negative (Atoi accepts -1, but spec wants real-world semver)
		{"0.5.0-dev", false}, // pre-release suffix
		{"0.5.0", true},
		{"v0.5.0", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			_, got := parseSemver(tc.in)
			if got != tc.ok {
				t.Errorf("parseSemver(%q) ok=%v, want %v", tc.in, got, tc.ok)
			}
		})
	}
}
