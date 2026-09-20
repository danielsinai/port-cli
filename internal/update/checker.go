package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/port-experimental/port-cli/internal/useragent"
)

const (
	cacheFile = ".port-cli-update-cache"
	cacheTTL  = 24 * time.Hour
)

var releasesURL = "https://api.github.com/repos/port-experimental/port-cli/releases/latest"

// CheckResult represents the result of an update check.
type CheckResult struct {
	LatestVersion   string
	CurrentVersion  string
	UpdateAvailable bool
	DownloadURL     string
	Error           error
}

// Checker checks for updates.
type Checker struct {
	httpClient *http.Client
}

// NewChecker creates a new update checker.
func NewChecker() *Checker {
	return &Checker{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CheckLatestVersion checks for the latest version on GitHub.
func (c *Checker) CheckLatestVersion(ctx context.Context, currentVersion string) (*CheckResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", releasesURL, nil)
	if err != nil {
		return &CheckResult{
			CurrentVersion: currentVersion,
			Error:          err,
		}, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", useragent.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &CheckResult{
			CurrentVersion: currentVersion,
			Error:          err,
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &CheckResult{
			CurrentVersion: currentVersion,
			Error:          fmt.Errorf("failed to fetch releases: %s", resp.Status),
		}, fmt.Errorf("failed to fetch releases: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &CheckResult{
			CurrentVersion: currentVersion,
			Error:          err,
		}, err
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}

	if err := json.Unmarshal(body, &release); err != nil {
		return &CheckResult{
			CurrentVersion: currentVersion,
			Error:          err,
		}, err
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	updateAvailable := compareVersions(currentVersion, latestVersion) < 0

	result := &CheckResult{
		LatestVersion:   latestVersion,
		CurrentVersion:  currentVersion,
		UpdateAvailable: updateAvailable,
		DownloadURL:     release.HTMLURL,
	}

	return result, nil
}

// compareVersions compares two version strings.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func compareVersions(v1, v2 string) int {
	// Simple version comparison - remove 'v' prefix and compare
	v1 = strings.TrimPrefix(strings.TrimSpace(v1), "v")
	v2 = strings.TrimPrefix(strings.TrimSpace(v2), "v")

	// Handle dev versions
	if v1 == "dev" {
		return -1
	}
	if v2 == "dev" {
		return 1
	}

	// Compare release numbers component by component, numerically. A string
	// comparison would rank "0.3.9" above "0.3.10".
	parts1 := strings.Split(releaseNumbers(v1), ".")
	parts2 := strings.Split(releaseNumbers(v2), ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		p1 := versionPart(parts1, i)
		p2 := versionPart(parts2, i)

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}

	return 0
}

// releaseNumbers strips any pre-release or build metadata suffix, leaving only
// the dot-separated release numbers (e.g. "0.3.7-5-gabc123" -> "0.3.7").
func releaseNumbers(v string) string {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		return v[:i]
	}
	return v
}

// versionPart returns parts[i] as a number, treating missing or unparsable
// components as 0 so that "1.0" and "1.0.0" compare equal.
func versionPart(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(parts[i])
	if err != nil {
		return 0
	}
	return n
}
