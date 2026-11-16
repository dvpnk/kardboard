package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// version is set when building a release, by scripts/dist from
// scripts/calculate_version.sh:
//
//	go build -ldflags "-X kardboard/internal/cli.version=v0.2.0"
var version string

// versionInfo is what kard knows about its own build. go build writes it
// into the program from the Git repository it was built in: the tag, or
// else the commit and its time. A version given at build time wins.
type versionInfo struct {
	Version  string `json:"version"`
	Commit   string `json:"commit,omitempty"`
	Time     string `json:"time,omitempty"`
	Modified bool   `json:"modified,omitempty"` // built with uncommitted changes
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Go       string `json:"go"`
}

func buildVersion() versionInfo {
	v := versionInfo{Version: "devel", OS: runtime.GOOS, Arch: runtime.GOARCH, Go: runtime.Version()}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = &debug.BuildInfo{}
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			v.Commit = s.Value
		case "vcs.time":
			v.Time = s.Value
		case "vcs.modified":
			v.Modified = s.Value == "true"
		}
	}
	// A tagged commit gives v1.2.3; anything else a pseudo-version, which
	// says less than commit and time do on their own.
	if m := strings.TrimSuffix(info.Main.Version, "+dirty"); m != "" && m != "(devel)" && !isPseudo(m) {
		v.Version = m
	}
	if version != "" {
		v.Version = version
	}
	return v
}

// isPseudo tells a pseudo-version such as v0.0.0-20261116101500-e33e55f6abcd
// from a tag.
func isPseudo(v string) bool {
	parts := strings.Split(v, "-")
	if len(parts) < 3 {
		return false
	}
	stamp := parts[len(parts)-2]
	if i := strings.LastIndex(stamp, "."); i >= 0 {
		stamp = stamp[i+1:]
	}
	return len(stamp) == 14 && strings.Trim(stamp, "0123456789") == ""
}

// String is the one line kard version prints.
func (v versionInfo) String() string {
	var b strings.Builder
	b.WriteString("kard " + v.Version)
	if v.Commit != "" {
		short := v.Commit
		if len(short) > 7 {
			short = short[:7]
		}
		b.WriteString(", commit " + short)
		if v.Modified {
			b.WriteString(" with uncommitted changes")
		}
		if len(v.Time) >= 10 {
			b.WriteString(" from " + v.Time[:10])
		}
	}
	fmt.Fprintf(&b, " (%s/%s, %s)", v.OS, v.Arch, v.Go)
	return b.String()
}

func (a *app) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Say which version of kard this is",
		Long: `Say which version of kard this is: the Git tag it was built from, or
else the commit, and for which system. kard --version says the same.`,
		Example: `  kard version
  kard version --json`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			v := buildVersion()
			if a.asJSON {
				return a.printJSON(v)
			}
			fmt.Fprintln(a.out, v)
			return nil
		},
	}
}
