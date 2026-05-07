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
