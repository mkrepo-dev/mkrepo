package internal

import "runtime/debug"

var (
	version       string
	revision      string
	buildDatetime string
)

type Version struct {
	Version       string
	GoVersion     string
	Revision      string
	BuildDatetime string
}

func ReadVersion() Version {
	var goVersion string
	info, ok := debug.ReadBuildInfo()
	if ok {
		goVersion = info.GoVersion
	}
	return Version{
		Version:       version,
		GoVersion:     goVersion,
		Revision:      revision,
		BuildDatetime: buildDatetime,
	}
}
