package version

import "runtime/debug"

var Version = "dev"
var Commit = "none"
var Date = "unknown"

func init() {
	ver, ok := readBuildInfo()
	if ok {
		Version = ver
	}
}

func readBuildInfo() (string, bool) {
	if Version != "dev" {
		return Version, false
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", false
	}
	if info.Main.Version != "" {
		Version = info.Main.Version
	}
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			Commit = kv.Value
		case "vcs.time":
			Date = kv.Value
		}
	}
	return Version, true
}


