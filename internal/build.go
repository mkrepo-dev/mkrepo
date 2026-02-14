package internal

import (
	"runtime"
	"time"
)

var (
	version       string
	revision      string
	buildDatetime string
)

type buildInfo struct {
	Version       string
	GoVersion     string
	Revision      string
	BuildDatetime time.Time
}

var Build = initBuildInfo()

func initBuildInfo() buildInfo {
	info := buildInfo{
		Version:   version,
		GoVersion: runtime.Version(),
		Revision:  revision,
	}

	if buildDatetime != "" {
		buildDatetime, err := time.Parse(time.RFC3339, buildDatetime)
		if err != nil {
			panic("invalid build datetime format")
		}
		info.BuildDatetime = buildDatetime
	}

	return info
}
