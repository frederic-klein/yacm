package index

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBackPANIndex_Lookup(t *testing.T) {
	// Arrange: Create mock MetaCPAN API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/download_url/JSON" {
			resp := BackPANResult{
				DownloadURL: "https://cpan.metacpan.org/authors/id/I/IS/ISHIGAKI/JSON-4.10.tar.gz",
				Version:     "4.10",
				Status:      "latest",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/v1/download_url/NonExistent" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	idx := NewBackPANIndex(t.TempDir())
	idx.apiURL = server.URL

	tests := []struct {
		name       string
		module     string
		version    string
		wantURL    string
		wantErr    bool
	}{
		{
			name:    "found module",
			module:  "JSON",
			version: "",
			wantURL: "https://cpan.metacpan.org/authors/id/I/IS/ISHIGAKI/JSON-4.10.tar.gz",
			wantErr: false,
		},
		{
			name:    "not found",
			module:  "NonExistent",
			version: "",
			wantURL: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result, err := idx.Lookup(tt.module, tt.version)

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("Lookup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result.DownloadURL != tt.wantURL {
				t.Errorf("DownloadURL = %q, want %q", result.DownloadURL, tt.wantURL)
			}
		})
	}
}

func TestBackPANIndex_LocalPath(t *testing.T) {
	backpanDir := "/tmp/backpan-modules"
	idx := NewBackPANIndex(backpanDir)

	tests := []struct {
		url  string
		want string
	}{
		{
			"https://cpan.metacpan.org/authors/id/I/IS/ISHIGAKI/JSON-4.10.tar.gz",
			"/tmp/backpan-modules/JSON-4.10.tar.gz",
		},
		{
			"https://backpan.perl.org/authors/id/M/MA/MAKAMAKA/JSON-2.0.tar.gz",
			"/tmp/backpan-modules/JSON-2.0.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := idx.LocalPath(tt.url)
			if got != tt.want {
				t.Errorf("LocalPath(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestBackPANIndex_EnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	backpanDir := filepath.Join(tmpDir, "backpan", "modules")
	idx := NewBackPANIndex(backpanDir)

	// Act
	err := idx.EnsureDir()

	// Assert
	if err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	info, err := os.Stat(backpanDir)
	if os.IsNotExist(err) {
		t.Error("directory was not created")
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestBackPANIndex_Dir(t *testing.T) {
	backpanDir := "/custom/backpan/dir"
	idx := NewBackPANIndex(backpanDir)

	if got := idx.Dir(); got != backpanDir {
		t.Errorf("Dir() = %q, want %q", got, backpanDir)
	}
}
