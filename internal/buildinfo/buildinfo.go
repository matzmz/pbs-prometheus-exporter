package buildinfo

import (
	"runtime/debug"

	"github.com/prometheus/common/version"
)

const (
	defaultVersion   = "dev"
	defaultBuildUser = "matzmz"
	unknownValue     = "unknown"
)

type vcsMetadata struct {
	mainVersion string
	revision    string
	commitTime  string
}

var metadata = readVCSMetadata()

var (
	Version   = defaultVersion
	Revision  = ""
	Branch    = ""
	BuildUser = defaultBuildUser
	BuildDate = ""
)

func init() {
	apply()
}

func apply() {
	version.Version = VersionValue()
	version.Revision = RevisionValue()
	version.Branch = BranchValue()
	version.BuildUser = BuildUserValue()
	version.BuildDate = BuildDateValue()
}

func Print(program string) string {
	return version.Print(program)
}

func Short() string {
	return VersionValue()
}

func VersionValue() string {
	if Version != "" && Version != defaultVersion {
		return Version
	}
	if metadata.mainVersion != "" {
		return metadata.mainVersion
	}
	if Version != "" {
		return Version
	}
	return defaultVersion
}

func RevisionValue() string {
	if Revision != "" {
		return Revision
	}
	if metadata.revision != "" {
		return metadata.revision
	}
	return version.GetRevision()
}

func BranchValue() string {
	if Branch == "" {
		return unknownValue
	}
	return Branch
}

func BuildUserValue() string {
	if BuildUser == "" || BuildUser == unknownValue {
		return defaultBuildUser
	}
	return BuildUser
}

func BuildDateValue() string {
	if BuildDate != "" && BuildDate != unknownValue {
		return BuildDate
	}
	if metadata.commitTime != "" {
		return metadata.commitTime
	}
	return unknownValue
}

func readVCSMetadata() vcsMetadata {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return vcsMetadata{}
	}

	meta := vcsMetadata{}
	if buildInfo.Main.Version != "" && buildInfo.Main.Version != "(devel)" {
		meta.mainVersion = buildInfo.Main.Version
	}

	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.revision":
			meta.revision = setting.Value
		case "vcs.time":
			meta.commitTime = setting.Value
		}
	}

	return meta
}
