package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

// These values are replaced by release builds through -ldflags. Development
// and go install builds fall back to the module and VCS metadata embedded by Go.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Modified  bool   `json:"modified"`
}

func Current() Info {
	info := Info{
		Version:   clean(Version, "dev"),
		Commit:    clean(Commit, "unknown"),
		BuildDate: clean(Date, "unknown"),
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
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
			if info.Commit == "unknown" {
				info.Commit = clean(setting.Value, "unknown")
			}
		case "vcs.time":
			if info.BuildDate == "unknown" {
				info.BuildDate = clean(setting.Value, "unknown")
			}
		case "vcs.modified":
			info.Modified = setting.Value == "true"
		}
	}
	return info
}

func clean(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
