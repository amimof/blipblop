package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/mod/semver"
)

// ParseVersion parses the provided string and ensures that it is a valid.
// Allowed values are either semver (prefixed with v), "latest" and "head". Case is ignored.
// If the version is valid, this function will resolv the version to a GitHub release name
// to ensure that the release exists.
func ParseVersion(in string) (string, error) {
	var url string
	in = strings.TrimSpace(in)
	in = strings.ToLower(in)

	// If user provides "head" then they mean "latest"
	if in == "head" {
		in = "latest"
	}

	// If user provides "latest"
	if in == "latest" {
		url = fmt.Sprintf("https://api.github.com/repos/amimof/blipblop/releases/%s", in)
	}

	// If user provides semver tag
	if semver.IsValid(in) {
		url = fmt.Sprintf("https://api.github.com/repos/amimof/blipblop/releases/tags/%s", in)
	}

	// If url is empty then user provided incorrect version string
	if url == "" {
		return "", fmt.Errorf("invalid version %s: allowed values are 'latest', 'head' or 'semver'", in)
	}

	httpClient := http.DefaultClient
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("error closing response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected response from server: %s", resp.Status)
	}

	var apiResponse struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		PublishedAt string `json:"published_at"`
	}

	err = json.Unmarshal(b, &apiResponse)
	if err != nil {
		return "", err
	}

	ver := apiResponse.TagName

	return ver, nil
}
