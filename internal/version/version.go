package version

import (
	"fmt"
	"runtime"
)

// These variables are set at build time using ldflags
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func Info() string {
	return fmt.Sprintf("skill-loop %s (%s) built on %s with %s",
		Version, Commit, Date, runtime.Version())
}
