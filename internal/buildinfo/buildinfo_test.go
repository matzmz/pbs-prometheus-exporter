package buildinfo

import (
	"strings"
	"testing"
)

func TestShortUsesVersionVariable(t *testing.T) {
	originalVersion := Version
	Version = "1.2.3"
	t.Cleanup(func() {
		Version = originalVersion
	})

	if got := Short(); got != "1.2.3" {
		t.Fatalf("Short() = %q, want %q", got, "1.2.3")
	}
}

func TestPrintIncludesBuildMetadata(t *testing.T) {
	originalVersion := Version
	originalRevision := Revision
	originalBranch := Branch
	originalBuildUser := BuildUser
	originalBuildDate := BuildDate
	t.Cleanup(func() {
		Version = originalVersion
		Revision = originalRevision
		Branch = originalBranch
		BuildUser = originalBuildUser
		BuildDate = originalBuildDate
		apply()
	})

	Version = "1.2.3"
	Revision = "abc1234"
	Branch = "main"
	BuildUser = "builder"
	BuildDate = "2026-07-03T12:34:56Z"
	apply()

	rendered := Print("pbs_exporter")

	for _, want := range []string{
		"pbs_exporter, version 1.2.3",
		"branch: main",
		"revision: abc1234",
		"build user:",
		"builder",
		"build date:",
		"2026-07-03T12:34:56Z",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Print() = %q, missing %q", rendered, want)
		}
	}
}

func TestRevisionValueFallsBackToUnknown(t *testing.T) {
	originalRevision := Revision
	t.Cleanup(func() {
		Revision = originalRevision
		apply()
	})

	Revision = ""
	apply()

	if got := RevisionValue(); got == "" {
		t.Fatal("RevisionValue() returned empty string")
	}
}

func TestBranchValueFallsBackToUnknown(t *testing.T) {
	originalBranch := Branch
	t.Cleanup(func() {
		Branch = originalBranch
		apply()
	})

	Branch = ""
	apply()

	if got := BranchValue(); got != "unknown" {
		t.Fatalf("BranchValue() = %q, want %q", got, "unknown")
	}
}
