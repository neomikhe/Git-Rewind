package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFallsBackToTheBuildInfo(t *testing.T) {
	cases := []struct {
		name                    string
		info                    debug.BuildInfo
		wantVersion, wantCommit string
		wantBuilt               string
	}{
		{
			name:        "go install from the module proxy",
			info:        debug.BuildInfo{Main: debug.Module{Version: "v0.1.1"}},
			wantVersion: "0.1.1", wantCommit: "none", wantBuilt: "unknown",
		},
		{
			name: "local build stamped from git",
			info: debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "32e508ea41a17d4da155b2689618a39275751d52"},
					{Key: "vcs.time", Value: "2026-11-24T23:43:47Z"},
				},
			},
			wantVersion: "dev", wantCommit: "32e508ea41a17d4da155b2689618a39275751d52", wantBuilt: "2026-11-24T23:43:47Z",
		},
		{
			name:        "no build information at all",
			info:        debug.BuildInfo{},
			wantVersion: "dev", wantCommit: "none", wantBuilt: "unknown",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, cm, d := fromBuildInfo(&c.info, "dev", "none", "unknown")
			if v != c.wantVersion || cm != c.wantCommit || d != c.wantBuilt {
				t.Errorf("fromBuildInfo = %q, %q, %q; want %q, %q, %q", v, cm, d, c.wantVersion, c.wantCommit, c.wantBuilt)
			}
		})
	}
}
