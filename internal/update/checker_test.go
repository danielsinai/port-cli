package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/port-experimental/port-cli/internal/useragent"
)

// TestChecker_UserAgent verifies that the GitHub release check sends a
// User-Agent header that begins with "port-cli/".
func TestChecker_UserAgent(t *testing.T) {
	useragent.SetVersion("test-version")
	t.Cleanup(func() { useragent.SetVersion("dev") })

	wantUA := useragent.String()

	var gotUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			TagName string `json:"tag_name"`
			HTMLURL string `json:"html_url"`
		}{TagName: "v1.0.0", HTMLURL: "https://example.com"})
	}))
	defer server.Close()

	// Temporarily override the releases URL to point at our test server.
	orig := releasesURL
	releasesURL = server.URL
	t.Cleanup(func() { releasesURL = orig })

	checker := NewChecker()
	_, err := checker.CheckLatestVersion(context.Background(), "1.0.0")
	if err != nil {
		t.Fatalf("CheckLatestVersion failed: %v", err)
	}

	if !strings.HasPrefix(gotUA, "port-cli/") {
		t.Errorf("User-Agent = %q, want prefix \"port-cli/\"", gotUA)
	}
	if gotUA != wantUA {
		t.Errorf("User-Agent = %q, want %q", gotUA, wantUA)
	}
}

// TestCompareVersions verifies that versions are compared numerically per
// component, not lexicographically (0.3.9 must sort below 0.3.10).
func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1   string
		v2   string
		want int
	}{
		{"0.3.9", "0.3.10", -1},
		{"0.3.10", "0.3.9", 1},
		{"1.9.0", "1.10.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"9.0.0", "10.0.0", -1},
		{"0.3.7", "0.3.7", 0},
		{"v0.3.7", "0.3.7", 0},
		{"1.0", "1.0.0", 0},
		{"0.9.0", "1.0.0", -1},
		{"1.0.0", "0.9.0", 1},
		{"0.3.7-5-gabc1234", "0.3.7", 0},
		{"0.3.7-5-gabc1234", "0.3.10", -1},
		{"dev", "0.3.7", -1},
		{"0.3.7", "dev", 1},
	}

	for _, tt := range tests {
		if got := compareVersions(tt.v1, tt.v2); got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}

// TestCheckLatestVersion_DoubleDigitPatch is a regression test for an update
// check that reported "latest" while a newer double-digit patch was released.
func TestCheckLatestVersion_DoubleDigitPatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			TagName string `json:"tag_name"`
			HTMLURL string `json:"html_url"`
		}{TagName: "v0.3.10", HTMLURL: "https://example.com"})
	}))
	defer server.Close()

	orig := releasesURL
	releasesURL = server.URL
	t.Cleanup(func() { releasesURL = orig })

	result, err := NewChecker().CheckLatestVersion(context.Background(), "0.3.9")
	if err != nil {
		t.Fatalf("CheckLatestVersion failed: %v", err)
	}
	if !result.UpdateAvailable {
		t.Errorf("UpdateAvailable = false for current 0.3.9 / latest 0.3.10, want true")
	}
}
