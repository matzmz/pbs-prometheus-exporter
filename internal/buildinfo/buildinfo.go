package buildinfo

import "github.com/prometheus/common/version"

var (
	Version   = "dev"
	Revision  = ""
	Branch    = ""
	BuildUser = "unknown"
	BuildDate = "unknown"
)

func init() {
	apply()
}

func apply() {
	version.Version = Version
	version.Revision = Revision
	version.Branch = Branch
	version.BuildUser = BuildUser
	version.BuildDate = BuildDate
}

func Print(program string) string {
	return version.Print(program)
}

func Short() string {
	return Version
}

func RevisionValue() string {
	return version.GetRevision()
}

func BranchValue() string {
	if Branch == "" {
		return "unknown"
	}
	return Branch
}
