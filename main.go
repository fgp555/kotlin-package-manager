package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"kotlin-package-manager/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "kt:", err)
		os.Exit(1)
	}
}
