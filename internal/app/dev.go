package app

import (
	"errors"
	"flag"
)

func runDev(args []string) error {
	if len(args) == 0 || args[0] != "dev" {
		return errors.New("usage: kpm run dev [-n|--noDevTools]")
	}
	flags := flag.NewFlagSet("run dev", flag.ContinueOnError)
	noDevTools := flags.Bool("noDevTools", false, "hide the Compose Hot Reload DevTools panel")
	flags.BoolVar(noDevTools, "n", false, "alias for --noDevTools")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: kpm run dev [-n|--noDevTools]")
	}

	p, err := findProject()
	if err != nil {
		return err
	}

	wrapperArgs := []string{"run", "-m", "desktopApp", "--compose-hot-reload"}
	if p.gradle {
		wrapperArgs = []string{"hotRun", "--auto"}
		if *noDevTools {
			wrapperArgs = append([]string{"-Pcompose.reload.devToolsHeadless=true"}, wrapperArgs...)
		}
	} else if *noDevTools {
		wrapperArgs = append(wrapperArgs, "--jvm-args=-Dcompose.reload.devToolsEnabled=false")
	}

	command, err := p.wrapperCommand(wrapperArgs...)
	if err != nil {
		return err
	}
	return command.Run()
}
