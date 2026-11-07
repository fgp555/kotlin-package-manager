package app

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type project struct {
	root, current, manifest, wrapper string
	gradle                           bool
}

func findProject() (project, error) {
	current, err := os.Getwd()
	if err != nil {
		return project{}, err
	}
	for dir := current; ; dir = filepath.Dir(dir) {
		toolchain, err := hasFile(dir, "project.yaml")
		if err != nil {
			return project{}, err
		}
		gradle, err := hasFile(dir, "settings.gradle.kts")
		if err != nil {
			return project{}, err
		}
		if toolchain && gradle {
			return project{}, fmt.Errorf("%s contains both project.yaml and settings.gradle.kts", dir)
		}
		if gradle {
			return project{dir, current, "build.gradle.kts", "gradlew", true}, nil
		}
		if toolchain {
			return project{dir, current, "module.yaml", "kotlin", false}, nil
		}
		if filepath.Dir(dir) == dir {
			return project{}, fmt.Errorf("project.yaml or settings.gradle.kts not found")
		}
	}
}

func hasFile(dir, name string) (bool, error) {
	info, err := os.Stat(filepath.Join(dir, name))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}

func (p project) target(module string, input *bufio.Reader) (string, error) {
	if module != "" {
		path := filepath.Join(p.root, module, p.manifest)
		_, err := os.Stat(path)
		return path, err
	}

	for dir := p.current; dir != p.root; dir = filepath.Dir(dir) {
		found, err := hasFile(dir, p.manifest)
		if err != nil {
			return "", err
		}
		if found {
			return filepath.Join(dir, p.manifest), nil
		}
	}
	found, err := hasFile(filepath.Join(p.root, "shared"), p.manifest)
	if err != nil {
		return "", err
	}
	if found {
		return filepath.Join(p.root, "shared", p.manifest), nil
	}

	entries, err := os.ReadDir(p.root)
	if err != nil {
		return "", err
	}
	var modules []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		found, err := hasFile(filepath.Join(p.root, entry.Name()), p.manifest)
		if err != nil {
			return "", err
		}
		if found {
			modules = append(modules, entry.Name())
		}
	}
	if len(modules) == 0 {
		found, err := hasFile(p.root, p.manifest)
		if err != nil {
			return "", err
		}
		if !found {
			return "", fmt.Errorf("no modules with %s found; specify -m", p.manifest)
		}
		modules = append(modules, ".")
	}
	chosen := modules[0]
	if len(modules) > 1 {
		chosen, err = selectOption("Select a module:", modules, input)
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(p.root, chosen, p.manifest), nil
}

func (p project) wrapperCommand(args ...string) (*exec.Cmd, error) {
	wrapper := p.wrapper
	if runtime.GOOS == "windows" {
		wrapper += ".bat"
	}
	path := filepath.Join(p.root, wrapper)
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	command := exec.Command(path, args...)
	if runtime.GOOS == "windows" {
		command = exec.Command("cmd.exe", append([]string{"/d", "/c", ".\\" + wrapper}, args...)...)
	}
	command.Dir = p.root
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command, nil
}
