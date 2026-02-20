package index

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const metacpanAPI = "https://fastapi.metacpan.org"

// BackPANIndex provides lookup for specific module versions via MetaCPAN API.
type BackPANIndex struct {
	apiURL     string
	backpanDir string
	client     *http.Client
}

// BackPANResult contains the download URL for a specific module version.
type BackPANResult struct {
	DownloadURL string `json:"download_url"`
	Version     string `json:"version"`
	Status      string `json:"status"`
}

// NewBackPANIndex creates a new BackPAN index.
func NewBackPANIndex(backpanDir string) *BackPANIndex {
	return &BackPANIndex{
		apiURL:     metacpanAPI,
		backpanDir: backpanDir,
		client:     &http.Client{},
	}
}

// Lookup queries MetaCPAN for a specific module version.
func (idx *BackPANIndex) Lookup(module, version string) (*BackPANResult, error) {
	// Build URL with version constraint
	apiURL := fmt.Sprintf("%s/v1/download_url/%s", idx.apiURL, url.PathEscape(module))
	if version != "" && version != "0" {
		apiURL = fmt.Sprintf("%s?version=%s", apiURL, url.QueryEscape(version))
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := idx.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying MetaCPAN: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("module %s version %s not found", module, version)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MetaCPAN API error: HTTP %d", resp.StatusCode)
	}

	var result BackPANResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &result, nil
}

// EnsureDir creates the backpan modules directory if needed.
func (idx *BackPANIndex) EnsureDir() error {
	return os.MkdirAll(idx.backpanDir, 0755)
}

// LocalPath returns the local path for a downloaded BackPAN module.
func (idx *BackPANIndex) LocalPath(downloadURL string) string {
	// Extract filename from URL
	parts := strings.Split(downloadURL, "/")
	filename := parts[len(parts)-1]
	return filepath.Join(idx.backpanDir, filename)
}

// Dir returns the backpan modules directory.
func (idx *BackPANIndex) Dir() string {
	return idx.backpanDir
}
