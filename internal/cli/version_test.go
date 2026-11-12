package cli

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	settings := func(kv ...string) []debug.BuildSetting {
		var s []debug.BuildSetting
		for i := 0; i < len(kv); i += 2 {
			s = append(s, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
		}
		return s
	}
	cases := []struct {
		name     string
		override string
		bi       *debug.BuildInfo
		want     string
	}{
		{"go install @tag", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, "v0.1.0"},
		{"local build after a tag", "", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1-0.20261112101500-0b3c6ef12345+dirty"}},
			"v0.1.1-0.20261112101500-0b3c6ef12345+dirty"},
		{"packager override", "0.1.0-1", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, "0.1.0-1"},
		{"devel with revision", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"},
			Settings: settings("vcs.revision", "0b3c6ef1234567890abc", "vcs.modified", "false")}, "devel-0b3c6ef12345"},
		{"devel dirty", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"},
			Settings: settings("vcs.revision", "0b3c6ef", "vcs.modified", "true")}, "devel-0b3c6ef-dirty"},
		{"nothing", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "devel"},
		{"no build info", "", nil, "devel"},
	}
	for _, c := range cases {
		if got := Version(c.override, c.bi); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestVersionLine(t *testing.T) {
	got := VersionLine("dev")
	if !strings.HasPrefix(got, "ng ") || !strings.Contains(got, "(go") {
		t.Errorf("got %q", got)
	}
}
