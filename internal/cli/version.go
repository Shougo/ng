package cli

import (
	"runtime/debug"
	"strings"
)

// Version returns ng's version for --version, --trace and bug reports.
//
// override is set with -ldflags "-X main.version=..." by packagers; when it
// is empty or "dev" the version comes from the build information Go embeds:
// the module version for "go install ...@v0.1.0", the version Go derives
// from git tags for a local build, or else the VCS revision.
func Version(override string, bi *debug.BuildInfo) string {
	if override != "" && override != "dev" {
		return override
	}
	if bi == nil {
		return "devel"
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev string
	dirty := false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "devel"
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	v := "devel-" + rev
	if dirty {
		v += "-dirty"
	}
	return v
}

// VersionLine is what "ng --version" prints.
func VersionLine(override string) string {
	bi, _ := debug.ReadBuildInfo()
	goVersion := ""
	if bi != nil {
		goVersion = " (" + strings.TrimSpace(bi.GoVersion) + ")"
	}
	return "ng " + Version(override, bi) + goVersion
}
