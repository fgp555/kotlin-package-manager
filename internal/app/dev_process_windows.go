package app

import (
	"fmt"
	"os/exec"
	"strconv"

	"golang.org/x/term"
)

func enableDevKeys(fd int) (func(), error) {
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return func() { _ = term.Restore(fd, state) }, nil
}

func prepareDevProcess(_ *exec.Cmd) {}

func stopDevProcess(command *exec.Cmd) error {
	output, err := exec.Command("taskkill.exe", "/pid", strconv.Itoa(command.Process.Pid), "/t", "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("stop development process: %w: %s", err, output)
	}
	return nil
}
