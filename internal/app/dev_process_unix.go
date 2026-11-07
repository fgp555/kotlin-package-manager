//go:build !windows

package app

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func enableDevKeys(_ int) (func(), error) {
	save := exec.Command("stty", "-g")
	save.Stdin = os.Stdin
	state, err := save.Output()
	if err != nil {
		return nil, err
	}
	enable := exec.Command("stty", "-icanon", "-echo", "min", "1", "time", "0")
	enable.Stdin = os.Stdin
	if err := enable.Run(); err != nil {
		return nil, err
	}
	return func() {
		restore := exec.Command("stty", strings.TrimSpace(string(state)))
		restore.Stdin = os.Stdin
		_ = restore.Run()
	}, nil
}
func prepareDevProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopDevProcess(command *exec.Cmd) error {
	return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}
