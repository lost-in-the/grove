package updatecheck

import "testing"

func TestDetectInstallFromPath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want InstallMethod
	}{
		{"homebrew apple silicon", "/opt/homebrew/Cellar/grove/0.6.0/bin/grove", InstallBrew},
		{"homebrew intel", "/usr/local/Cellar/grove/0.6.0/bin/grove", InstallBrew},
		{"go install", "/Users/leah/go/bin/grove", InstallGoInstall},
		{"binary download", "/usr/local/bin/grove", InstallBinary},
		{"empty", "", InstallUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectInstallFromPath(tc.path); got != tc.want {
				t.Errorf("detectInstallFromPath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestUpdateCommand(t *testing.T) {
	cases := []struct {
		method InstallMethod
		want   string
	}{
		{InstallBrew, "brew upgrade lost-in-the/tap/grove"},
		{InstallGoInstall, "go install github.com/lost-in-the/grove/cmd/grove@latest"},
		{InstallBinary, "Visit https://github.com/lost-in-the/grove/releases for the latest binary"},
		{InstallUnknown, "Visit https://github.com/lost-in-the/grove/releases for the latest binary"},
	}
	for _, tc := range cases {
		t.Run(tc.method.String(), func(t *testing.T) {
			if got := UpdateCommand(tc.method); got != tc.want {
				t.Errorf("UpdateCommand(%v) = %q, want %q", tc.method, got, tc.want)
			}
		})
	}
}
