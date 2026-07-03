package buildinfo

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseWorkflowUsesBuildInfoLdflags(t *testing.T) {
	content, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	workflow := string(content)

	for _, want := range []string{
		"scripts/ldflags.sh",
		"-ldflags \"$LDFLAGS\"",
		"scripts/verify-buildinfo.sh",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("release workflow does not contain %q", want)
		}
	}
}
