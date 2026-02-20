package index

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCPANIndex_Lookup_NotLoaded(t *testing.T) {
	idx := NewCPANIndex("https://cpan.metacpan.org", t.TempDir())

	_, found := idx.Lookup("JSON")
	if found {
		t.Error("Lookup() should return false when index not loaded")
	}
}

func TestCPANIndex_ParseCache(t *testing.T) {
	cacheDir := t.TempDir()
	cacheFile := filepath.Join(cacheDir, "02packages.details.txt")

	content := `File:         02packages.details.txt
URL:          http://www.perl.com/CPAN/modules/02packages.details.txt
Description:  Package names found in directory

JSON	2.97001	M/MA/MAKAMAKA/JSON-2.97001.tar.gz
Moo	2.005005	H/HA/HAARG/Moo-2.005005.tar.gz
Module::With::Undef	undef	A/AU/AUTHOR/Module-With-Undef-1.0.tar.gz
`
	if err := os.WriteFile(cacheFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	idx := NewCPANIndex("https://cpan.metacpan.org", cacheDir)
	if err := idx.parseCache(); err != nil {
		t.Fatalf("parseCache() error = %v", err)
	}

	tests := []struct {
		module      string
		wantVersion string
		wantPath    string
		wantFound   bool
	}{
		{"JSON", "2.97001", "M/MA/MAKAMAKA/JSON-2.97001.tar.gz", true},
		{"Moo", "2.005005", "H/HA/HAARG/Moo-2.005005.tar.gz", true},
		{"Module::With::Undef", "undef", "A/AU/AUTHOR/Module-With-Undef-1.0.tar.gz", true},
		{"NonExistent", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			entry, found := idx.Lookup(tt.module)
			if found != tt.wantFound {
				t.Errorf("Lookup(%q) found = %v, want %v", tt.module, found, tt.wantFound)
			}
			if found {
				if entry.Version != tt.wantVersion {
					t.Errorf("version = %q, want %q", entry.Version, tt.wantVersion)
				}
				if entry.Pathname != tt.wantPath {
					t.Errorf("pathname = %q, want %q", entry.Pathname, tt.wantPath)
				}
			}
		})
	}
}

func TestCPANIndex_Download(t *testing.T) {
	// Arrange: Create a mock server with properly gzipped content
	var gzippedContent bytes.Buffer
	gw := gzip.NewWriter(&gzippedContent)
	gw.Write([]byte("File: 02packages\n\nJSON\t2.0\tM/MA/MAKAMAKA/JSON-2.0.tar.gz\n"))
	gw.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/02packages.details.txt.gz" {
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(gzippedContent.Bytes())
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	idx := NewCPANIndex(server.URL, cacheDir)

	// Act
	err := idx.download()

	// Assert
	if err != nil {
		t.Fatalf("download() error = %v", err)
	}

	if _, err := os.Stat(idx.cacheFile); os.IsNotExist(err) {
		t.Error("cache file was not created")
	}
}

func TestCPANIndex_Mirror(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://cpan.metacpan.org", "https://cpan.metacpan.org"},
		{"https://cpan.metacpan.org/", "https://cpan.metacpan.org"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			idx := NewCPANIndex(tt.input, t.TempDir())
			if got := idx.Mirror(); got != tt.want {
				t.Errorf("Mirror() = %q, want %q", got, tt.want)
			}
		})
	}
}
