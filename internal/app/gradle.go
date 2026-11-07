package app

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// The editor recognizes existing dependency blocks and leaves the rest of the script intact.
var gradleToken = regexp.MustCompile(`(?s)"""(?:.*?)"""|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|//[^\r\n]*|/\*.*?\*/|[A-Za-z_][A-Za-z_0-9]*|[^\s]`)

type scriptToken struct {
	value string
	start int
}

type dependencyBlock struct {
	open, close int
	label       string
}

func addGradleDependencies(path, sourceSet string, coordinates []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	source := string(data)
	tokens := tokenizeGradle(source)
	block, err := findDependencyBlock(tokens, sourceSet)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	fmt.Printf("Target: %s → %s\n", path, block.label)

	versions := make(map[string]string)
	depth := 0
	for i := block.open + 1; i < block.close; i++ {
		switch tokens[i].value {
		case "{":
			depth++
		case "}":
			depth--
		case "(":
			if depth == 0 && i+2 < block.close && tokens[i+2].value == ")" {
				value := tokens[i+1].value
				if len(value) > 2 && value[0] == '"' && value[len(value)-1] == '"' {
					coordinate := value[1 : len(value)-1]
					if key, ok := mavenKey(coordinate); ok {
						if existing, exists := versions[key]; exists && existing != coordinate {
							return fmt.Errorf("%s already declares conflicting versions of %s", path, key)
						}
						versions[key] = coordinate
					}
				}
			}
		}
	}

	var added []string
	for _, coordinate := range coordinates {
		key, _ := mavenKey(coordinate)
		if existing, found := versions[key]; found {
			if existing != coordinate {
				return fmt.Errorf("%s already declares %s in %s; resolve the version before adding %s", path, existing, block.label, coordinate)
			}
			continue
		}
		versions[key] = coordinate
		added = append(added, coordinate)
	}
	if len(added) == 0 {
		fmt.Println("Dependencies already present.")
		return nil
	}

	open := tokens[block.open].start
	lineStart := strings.LastIndex(source[:open], "\n") + 1
	line := source[lineStart:open]
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	newline := "\n"
	if strings.Contains(source, "\r\n") {
		newline = "\r\n"
	}

	// Insert after the opening brace, even when the existing block is on one line.
	insert := newline
	for _, coordinate := range added {
		insert += indent + "    implementation(\"" + coordinate + "\")" + newline
	}
	rest := source[open+1:]
	if strings.HasPrefix(rest, newline) {
		rest = strings.TrimPrefix(rest, newline)
	} else {
		rest = indent + strings.TrimLeft(rest, " \t")
	}
	if err := replaceFile(path, []byte(source[:open+1]+insert+rest)); err != nil {
		return err
	}
	for _, coordinate := range added {
		fmt.Println("Added", coordinate)
	}
	return nil
}

func mavenKey(coordinate string) (string, bool) {
	group, rest, ok := strings.Cut(coordinate, ":")
	artifact, _, versioned := strings.Cut(rest, ":")
	if !ok || !versioned || !coordinatePart.MatchString(group) || !coordinatePart.MatchString(artifact) {
		return "", false
	}
	return group + ":" + artifact, true
}

func tokenizeGradle(source string) []scriptToken {
	var tokens []scriptToken
	for _, span := range gradleToken.FindAllStringIndex(source, -1) {
		value := source[span[0]:span[1]]
		if !strings.HasPrefix(value, "//") && !strings.HasPrefix(value, "/*") {
			tokens = append(tokens, scriptToken{value, span[0]})
		}
	}
	return tokens
}

func findDependencyBlock(tokens []scriptToken, sourceSet string) (dependencyBlock, error) {
	target := sourceSet
	if target == "" {
		target = "commonMain"
	}
	var stack []int
	var topLevel, sourceBlocks []dependencyBlock
	for index, token := range tokens {
		switch token.value {
		case "{":
			stack = append(stack, index)
		case "}":
			if len(stack) == 0 {
				return dependencyBlock{}, fmt.Errorf("unbalanced Gradle script")
			}
			open := stack[len(stack)-1]
			if open > 0 && tokens[open-1].value == "dependencies" {
				if len(stack) == 1 {
					topLevel = append(topLevel, dependencyBlock{open, index, "dependencies"})
				}
				if len(stack) == 3 && stack[0] > 0 && stack[1] > 0 && open >= 3 &&
					tokens[stack[0]-1].value == "kotlin" && tokens[stack[1]-1].value == "sourceSets" &&
					tokens[open-2].value == "." && tokens[open-3].value == target {
					sourceBlocks = append(sourceBlocks, dependencyBlock{open, index, target + ".dependencies"})
				}
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return dependencyBlock{}, fmt.Errorf("unbalanced Gradle script")
	}
	blocks := sourceBlocks
	if sourceSet == "" && len(blocks) == 0 {
		blocks = topLevel
	}
	if len(blocks) != 1 {
		return dependencyBlock{}, fmt.Errorf("expected one existing dependencies block; use --source-set with an existing kotlin { sourceSets { <name>.dependencies { ... } } } block")
	}
	return blocks[0], nil
}
