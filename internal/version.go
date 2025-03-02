package internal

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/FilipSolich/mkrepo/internal/log"
)

var (
	version       string
	revision      string
	buildDatetime string
)

var UserAgent = fmt.Sprintf("mkrepo/%s", version)

type Version struct {
	Version       string
	GoVersion     string
	Revision      string
	BuildDatetime time.Time
}

func ReadVersion() Version {
	var goVersion string
	info, ok := debug.ReadBuildInfo()
	if ok {
		goVersion = info.GoVersion
	}
	buildDatetime, err := time.Parse(time.RFC3339, buildDatetime)
	if err != nil {
		slog.Warn("Failed to parse build datetime", log.Err(err))
	}
	return Version{
		Version:       version,
		GoVersion:     goVersion,
		Revision:      revision,
		BuildDatetime: buildDatetime,
	}
}
