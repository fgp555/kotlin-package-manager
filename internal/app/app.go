package app

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const version = "0.1.0"
const usage = "Usage: kpm add <artifact|group:artifact[:version]>... [-m module]\n       kpm version\n       kpm help"

func Run(args []string) error {
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") || args[0] == "--help" || args[0] == "-h" {
		fmt.Println(usage)
		return nil
	}
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Println("kpm", version)
		return nil
	}
	if args[0] != "add" || len(args) < 2 {
		return errors.New(usage)
	}
	if args[1] == "-h" || args[1] == "--help" {
		fmt.Println(usage)
		return nil
	}

	flags := flag.NewFlagSet("add", flag.ContinueOnError)
	module := flags.String("m", "shared", "module directory relative to the project root")
	names := args[1:]
	optionStart := slices.IndexFunc(names, func(arg string) bool { return strings.HasPrefix(arg, "-") })
	var optionArgs []string
	if optionStart >= 0 {
		names, optionArgs = names[:optionStart], names[optionStart:]
	}
	if err := flags.Parse(optionArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if len(names) == 0 || flags.NArg() != 0 || !filepath.IsLocal(*module) {
		return errors.New(usage)
	}

	root, err := projectRoot()
	if err != nil {
		return err
	}
	manifest := filepath.Join(root, *module, "module.yaml")
	if _, err := os.Stat(manifest); err != nil {
		return err
	}
	input := bufio.NewReader(os.Stdin)
	dependencies := make([]string, 0, len(names))
	for _, name := range names {
		coordinate, err := resolve(name, input)
		if err != nil {
			return err
		}
		dependencies = append(dependencies, coordinate)
	}
	if err := addDependencies(manifest, dependencies); err != nil {
		return err
	}
	return build(root)
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		_, err := os.Stat(filepath.Join(dir, "project.yaml"))
		if err == nil {
			return dir, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("project.yaml not found; run inside a Kotlin Toolchain project")
		}
		dir = parent
	}
}

func addDependencies(path string, coordinates []string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(source, &document); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s must contain a YAML mapping", path)
	}
	root := document.Content[0]
	var dependencies *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "dependencies" {
			if dependencies != nil {
				return fmt.Errorf("%s has duplicate dependencies blocks", path)
			}
			dependencies = root.Content[i+1]
		}
	}
	if dependencies == nil {
		dependencies = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "dependencies"}, dependencies)
	}
	if dependencies.Kind == yaml.ScalarNode && dependencies.Tag == "!!null" {
		dependencies.Kind, dependencies.Tag, dependencies.Value = yaml.SequenceNode, "!!seq", ""
	}
	if dependencies.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s: dependencies must be a YAML list", path)
	}

	var added []string
	for _, coordinate := range coordinates {
		groupArtifact := strings.Join(strings.Split(coordinate, ":")[:2], ":")
		found := false
		for _, entry := range dependencies.Content {
			key := entry
			if entry.Kind == yaml.MappingNode && len(entry.Content) > 0 {
				key = entry.Content[0]
			}
			if key.Value == coordinate {
				found = true
			} else if key.Value == groupArtifact || strings.HasPrefix(key.Value, groupArtifact+":") {
				return fmt.Errorf("%s already declares %s; resolve the version before adding %s", path, key.Value, coordinate)
			}
		}
		if !found {
			dependencies.Content = append(dependencies.Content, &yaml.Node{
				Kind: yaml.ScalarNode, Tag: "!!str", Value: coordinate, Style: yaml.DoubleQuotedStyle,
			})
			added = append(added, coordinate)
		}
	}
	if len(added) == 0 {
		fmt.Println("Dependencies already present.")
		return nil
	}

	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	if err := replaceFile(path, output.Bytes()); err != nil {
		return err
	}
	for _, coordinate := range added {
		fmt.Println("Added", coordinate)
	}
	return nil
}

func replaceFile(path string, content []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".kpm-module-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := temp.Write(content); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func build(root string) error {
	wrapper := "kotlin"
	if runtime.GOOS == "windows" {
		wrapper = "kotlin.bat"
	}
	path := filepath.Join(root, wrapper)
	if _, err := os.Stat(path); err != nil {
		return err
	}
	command := exec.Command(path, "build")
	if runtime.GOOS == "windows" {
		command = exec.Command("cmd.exe", "/d", "/c", ".\\kotlin.bat", "build")
	}
	command.Dir = root
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}
