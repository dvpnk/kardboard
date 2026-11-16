package cli

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	code, out, _ := kard(t, "version")
	if code != ExitOK || !strings.HasPrefix(out, "kard ") || !strings.Contains(out, runtime.GOOS+"/"+runtime.GOARCH) {
		t.Fatalf("kard version = %d %q", code, out)
	}
	if code, flag, _ := kard(t, "--version"); code != ExitOK || flag != out {
		t.Errorf("kard --version = %d %q, want %q", code, flag, out)
	}
	code, out, _ = kard(t, "version", "--json")
	var v versionInfo
	if err := json.Unmarshal([]byte(out), &v); code != ExitOK || err != nil || v.Version == "" || v.OS != runtime.GOOS {
		t.Fatalf("kard version --json = %d %q (%v)", code, out, err)
	}
}

func TestIsPseudo(t *testing.T) {
	for v, want := range map[string]bool{
		"v1.2.3":                               false,
		"v1.2.3-rc.1":                          false,
		"v0.0.0-20261116101500-e33e55f6abcd":   true,
		"v1.2.4-0.20261116101500-e33e55f6abcd": true,
	} {
		if got := isPseudo(v); got != want {
			t.Errorf("isPseudo(%q) = %v", v, got)
		}
	}
}
