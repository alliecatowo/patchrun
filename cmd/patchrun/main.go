// Command patchrun runs a command in a disposable Git worktree and returns
// the resulting patch.
package main

import (
	"context"
	"os"

	"github.com/alliecatowo/patchrun/internal/app"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(app.Run(context.Background(), os.Args[1:], app.DefaultIO(), version))
}
