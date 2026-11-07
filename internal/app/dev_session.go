package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"golang.org/x/term"
)

var errRestartDev = errors.New("restart dev session")

func runDevSessions(p project, args []string, follow bool) error {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return runDevSession(p, args, follow, nil, nil)
	}
	restore, err := enableDevKeys(fd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kpm: keyboard shortcuts unavailable:", err)
		return runDevSession(p, args, follow, nil, nil)
	}
	defer restore()
	if p.gradle {
		// A single-use daemon keeps this application's processes scoped to this session.
		args = append(args, "--no-daemon")
	}

	keys := make(chan byte)
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)
	go func() {
		defer close(keys)
		var buffer [1]byte
		for {
			if _, err := os.Stdin.Read(buffer[:]); err != nil {
				return
			}
			switch key := buffer[0]; key {
			case 'r', 'R', 3:
				keys <- key
			}
		}
	}()
	fmt.Fprintln(os.Stderr, "kpm: R restart · Ctrl+C stop")
	for {
		err := runDevSession(p, args, follow, keys, interrupted)
		if !errors.Is(err, errRestartDev) {
			return err
		}
		fmt.Fprintln(os.Stderr, "kpm: restarting Compose Desktop...")
	}
}

func runDevSession(p project, args []string, follow bool, keys <-chan byte, interrupted <-chan os.Signal) error {
	command, err := p.wrapperCommand(args...)
	if err != nil {
		return err
	}
	run := command.Run
	if keys != nil {
		run = func() error { return runDevProcess(command, keys, interrupted) }
	}
	if follow {
		return runGradleDev(p.root, run)
	}
	return run()
}

func runDevProcess(command *exec.Cmd, keys <-chan byte, interrupted <-chan os.Signal) error {
	command.Stdin = nil
	prepareDevProcess(command)
	if err := command.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()

	var restart bool
	select {
	case err := <-exited:
		return err
	case key, ok := <-keys:
		restart = ok && (key == 'r' || key == 'R')
	case <-interrupted:
	}

	if err := stopDevProcess(command); err != nil {
		select {
		case <-exited:
		default:
			return err
		}
	} else {
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			return errors.New("timed out waiting for the development process to stop")
		}
	}
	if restart {
		return errRestartDev
	}
	return nil
}
