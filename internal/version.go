package internal

import "runtime/debug"

var (
	version       string
	buildDatetime string
)

type Version struct {
	Version       string
	GoVersion     string
	Revision      string
	BuildDatetime string
}

func ReadVersion() Version {
	var goVersion, revision string
	info, ok := debug.ReadBuildInfo()
	if ok {
		goVersion = info.GoVersion
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value[:7]
			}
		}
	}
	return Version{
		Version:       version,
		GoVersion:     goVersion,
		Revision:      revision,
		BuildDatetime: buildDatetime,
	}
}
