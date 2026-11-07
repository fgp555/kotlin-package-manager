package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"kotlin-package-manager/kpm/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "kpm:", err)
		os.Exit(1)
	}
}
