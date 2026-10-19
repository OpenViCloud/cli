package main

import (
	"runtime/debug"
	"testing"
)

func buildInfoWith(version string) func() (*debug.BuildInfo, bool) {
	return func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: version}}, true
	}
}

func TestBuildVersion_ReleaseBuild_KeepsTheLinkedVersion(t *testing.T) {
	got := buildVersion("0.1.0", buildInfoWith("v9.9.9"))
	if got != "0.1.0" {
		t.Fatalf("got %q, want 0.1.0", got)
	}
}

func TestBuildVersion_GoInstallOfATag_ShowsTheModuleVersion(t *testing.T) {
	got := buildVersion("dev", buildInfoWith("v0.1.0"))
	if got != "0.1.0" {
		t.Fatalf("got %q, want 0.1.0", got)
	}
}

func TestBuildVersion_LocalBuild_StaysDev(t *testing.T) {
	got := buildVersion("dev", buildInfoWith("(devel)"))
	if got != "dev" {
		t.Fatalf("got %q, want dev", got)
	}
}

func TestBuildVersion_NoBuildInfo_StaysDev(t *testing.T) {
	got := buildVersion("dev", func() (*debug.BuildInfo, bool) { return nil, false })
	if got != "dev" {
		t.Fatalf("got %q, want dev", got)
	}
}
