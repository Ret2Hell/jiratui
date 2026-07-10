package version

import "runtime/debug"

// AppName is the CLI and application name.
const AppName = "jiratui"

// Info describes build and version metadata.
type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Built   string `json:"built"`
}

// Build metadata set by release tooling.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Get returns the current build and version metadata.
func Get() Info {
	info := Info{
		Name:    AppName,
		Version: Version,
		Commit:  Commit,
		Built:   Date,
	}

	build, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}

	if info.Version == "dev" && build.Main.Version != "" && build.Main.Version != "(devel)" {
		info.Version = build.Main.Version
	}

	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "none" {
				info.Commit = setting.Value
			}
		case "vcs.time":
			if info.Built == "unknown" {
				info.Built = setting.Value
			}
		}
	}

	return info
}
