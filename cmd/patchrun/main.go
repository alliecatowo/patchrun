// Command patchrun runs a command in a disposable Git worktree and returns
// the resulting patch.
package main

import (
	"context"
	"os"
	"runtime/debug"
	"strings"

	"github.com/alliecatowo/patchrun/internal/app"
)

// version is set at build time via -ldflags "-X main.version=...". Builds
// without it (go install, go build) report the module version when known.
var version = buildVersion()

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return "dev"
}

// realMain is the os.Exit-free entry point. It exists so tests can drive the
// CLI without terminating the test process.
func realMain(args []string, io app.IO) int {
	return app.Run(context.Background(), args, io, version)
}

func main() {
	os.Exit(realMain(os.Args[1:], app.DefaultIO()))
}
