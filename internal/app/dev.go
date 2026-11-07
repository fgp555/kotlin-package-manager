package app

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const devUsage = "kpm run dev [--devTools] [--noLogs]"

func runDev(args []string) error {
	if len(args) == 0 || args[0] != "dev" {
		return errors.New("usage: " + devUsage)
	}
	flags := flag.NewFlagSet("run dev", flag.ContinueOnError)
	showDevTools := flags.Bool("devTools", false, "show the Compose Hot Reload DevTools panel")
	noLogs := flags.Bool("noLogs", false, "skip following the Gradle Hot Reload log")
	noDevTools := flags.Bool("noDevTools", false, "legacy flag for hiding the panel (the default)")
	flags.BoolVar(noDevTools, "n", false, "alias for --noDevTools")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: " + devUsage)
	}

	if *showDevTools && *noDevTools {
		return errors.New("--devTools conflicts with --noDevTools and -n")
	}

	p, err := findProject()
	if err != nil {
		return err
	}

	wrapperArgs := []string{"run", "-m", "desktopApp", "--compose-hot-reload"}
	if p.gradle {
		wrapperArgs = []string{"hotRun", "--auto", "--console=plain"}
		if !*showDevTools {
			wrapperArgs = append([]string{"-Pcompose.reload.devToolsHeadless=true"}, wrapperArgs...)
		}
	} else if !*showDevTools {
		wrapperArgs = append(wrapperArgs, "--jvm-args=-Dcompose.reload.devToolsEnabled=false")
	}

	command, err := p.wrapperCommand(wrapperArgs...)
	if err != nil {
		return err
	}
	if p.gradle && !*noLogs {
		return runGradleDev(p.root, command)
	}
	return command.Run()
}

// Record existing logs before starting Gradle so the terminal only shows this
// session's output, including a log created after the wrapper starts.
func runGradleDev(root string, command *exec.Cmd) error {
	paths, err := hotReloadLogPaths(root)
	if err != nil {
		return err
	}
	cursors := make([]logCursor, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		cursor := logCursor{path: path, info: info}
		if info != nil {
			cursor.offset = info.Size()
		}
		cursors = append(cursors, cursor)
	}

	if err := command.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		var selected *logCursor
		err := followLog(done, func() error {
			if selected == nil {
				var newest time.Time
				for i := range cursors {
					cursor := &cursors[i]
					info, err := os.Stat(cursor.path)
					if os.IsNotExist(err) {
						continue
					}
					if err != nil {
						return err
					}
					if info.Size() == 0 || (os.SameFile(cursor.info, info) && cursor.offset == info.Size()) {
						continue
					}
					if selected == nil || info.ModTime().After(newest) {
						selected, newest = cursor, info.ModTime()
					}
				}
				if selected == nil {
					return nil
				}
			}
			return selected.drain()
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "kpm: Hot Reload log:", err)
		}
	}()
	err = command.Wait()
	close(done)
	<-finished
	return err
}
