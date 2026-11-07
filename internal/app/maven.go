package app

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	centralClient  = &http.Client{Timeout: 15 * time.Second}
	coordinatePart = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.+\-]*$`)
	stableVersion  = regexp.MustCompile(`(?i)^[0-9]+(\.[0-9]+)*([.\-](final|ga|release))?$`)
)

func resolve(name string, input *bufio.Reader) (string, error) {
	parts := strings.Split(name, ":")
	if len(parts) > 3 {
		return "", fmt.Errorf("invalid Maven coordinate %q", name)
	}
	for _, part := range parts {
		if !coordinatePart.MatchString(part) {
			return "", fmt.Errorf("invalid Maven coordinate %q", name)
		}
	}
	if len(parts) == 3 {
		return name, nil
	}

	group, artifact := parts[0], parts[len(parts)-1]
	if len(parts) == 1 {
		var err error
		group, err = selectGroup(artifact, input)
		if err != nil {
			return "", err
		}
	}
	for _, segment := range strings.Split(group, ".") {
		if segment == "" {
			return "", fmt.Errorf("invalid Maven group %q", group)
		}
	}
	version, err := latestStable(group, artifact)
	if err != nil {
		return "", err
	}
	return group + ":" + artifact + ":" + version, nil
}

func selectGroup(artifact string, input *bufio.Reader) (string, error) {
	query := url.Values{"q": {`a:"` + artifact + `"`}, "rows": {"100"}, "wt": {"json"}}
	var result struct {
		Response struct {
			NumFound int
			Docs     []struct {
				Group    string `json:"g"`
				Artifact string `json:"a"`
			}
		}
	}
	address := "https://search.maven.org/solrsearch/select?" + query.Encode()
	if err := readCentral(address, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(&result)
	}); err != nil {
		return "", err
	}
	if result.Response.NumFound > len(result.Response.Docs) {
		return "", fmt.Errorf("too many matches for %s; specify group:artifact", artifact)
	}

	groups := make(map[string]bool)
	for _, doc := range result.Response.Docs {
		if doc.Artifact == artifact && coordinatePart.MatchString(doc.Group) {
			groups[doc.Group] = true
		}
	}
	switch len(groups) {
	case 0:
		return "", fmt.Errorf("no exact Maven Central match for %s; use group:artifact:version for other repositories", artifact)
	case 1:
		for group := range groups {
			return group, nil
		}
	}

	options := make([]string, 0, len(groups))
	for group := range groups {
		options = append(options, group)
	}
	sort.Strings(options)
	fmt.Fprintf(os.Stderr, "Multiple Maven Central artifacts match %s:\n", artifact)
	for index, group := range options {
		fmt.Fprintf(os.Stderr, "  %d. %s:%s\n", index+1, group, artifact)
	}
	fmt.Fprint(os.Stderr, "Select a number: ")
	line, err := input.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(options) {
		return "", fmt.Errorf("invalid selection for %s; specify group:artifact to skip the prompt", artifact)
	}
	return options[choice-1], nil
}

func latestStable(group, artifact string) (string, error) {
	var metadata struct {
		Release  string   `xml:"versioning>release"`
		Versions []string `xml:"versioning>versions>version"`
	}
	address := "https://repo.maven.apache.org/maven2/" +
		strings.ReplaceAll(group, ".", "/") + "/" + artifact + "/maven-metadata.xml"
	if err := readCentral(address, func(r io.Reader) error {
		return xml.NewDecoder(r).Decode(&metadata)
	}); err != nil {
		return "", err
	}
	if stableVersion.MatchString(metadata.Release) {
		return metadata.Release, nil
	}
	for index := len(metadata.Versions) - 1; index >= 0; index-- {
		if version := metadata.Versions[index]; stableVersion.MatchString(version) {
			return version, nil
		}
	}
	return "", fmt.Errorf("no recognized stable version for %s:%s; specify group:artifact:version", group, artifact)
}

func readCentral(address string, decode func(io.Reader) error) error {
	response, err := centralClient.Get(address)
	if err != nil {
		return fmt.Errorf("Maven Central: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Maven Central returned %s for %s", response.Status, address)
	}
	if err := decode(io.LimitReader(response.Body, 2<<20)); err != nil {
		return fmt.Errorf("invalid Maven Central response: %w", err)
	}
	return nil
}
