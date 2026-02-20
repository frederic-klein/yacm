package downloader

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloader_Download_SingleFile(t *testing.T) {
	// Arrange
	content := []byte("test tarball content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	dl := NewDownloader(2, cacheDir)
	destPath := filepath.Join(cacheDir, "test.tar.gz")

	jobs := []Job{{
		URL:      server.URL + "/test.tar.gz",
		DestPath: destPath,
		Source:   "cpan",
	}}

	// Act
	results := dl.Download(jobs)

	// Assert
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("Download() error = %v", results[0].Error)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("file content = %q, want %q", data, content)
	}
}

func TestDownloader_Download_Cached(t *testing.T) {
	// Arrange: Pre-create the file
	cacheDir := t.TempDir()
	destPath := filepath.Join(cacheDir, "cached.tar.gz")
	if err := os.WriteFile(destPath, []byte("cached"), 0644); err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Write([]byte("new content"))
	}))
	defer server.Close()

	dl := NewDownloader(1, cacheDir)
	jobs := []Job{{
		URL:      server.URL + "/cached.tar.gz",
		DestPath: destPath,
		Source:   "cpan",
	}}

	// Act
	results := dl.Download(jobs)

	// Assert
	if results[0].Error != nil {
		t.Errorf("Download() error = %v", results[0].Error)
	}
	if requestCount != 0 {
		t.Errorf("server was called %d times, want 0 (should use cache)", requestCount)
	}

	data, _ := os.ReadFile(destPath)
	if string(data) != "cached" {
		t.Error("cached file was overwritten")
	}
}

func TestDownloader_Download_HTTPError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	dl := NewDownloader(1, cacheDir)
	jobs := []Job{{
		URL:      server.URL + "/notfound.tar.gz",
		DestPath: filepath.Join(cacheDir, "notfound.tar.gz"),
		Source:   "cpan",
	}}

	// Act
	results := dl.Download(jobs)

	// Assert
	if results[0].Error == nil {
		t.Error("Download() should return error for 404")
	}
}

func TestDownloader_Download_Parallel(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("content for " + r.URL.Path))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	dl := NewDownloader(3, cacheDir)

	jobs := []Job{
		{URL: server.URL + "/file1.tar.gz", DestPath: filepath.Join(cacheDir, "file1.tar.gz"), Source: "cpan"},
		{URL: server.URL + "/file2.tar.gz", DestPath: filepath.Join(cacheDir, "file2.tar.gz"), Source: "cpan"},
		{URL: server.URL + "/file3.tar.gz", DestPath: filepath.Join(cacheDir, "file3.tar.gz"), Source: "cpan"},
	}

	// Act
	results := dl.Download(jobs)

	// Assert
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	for _, r := range results {
		if r.Error != nil {
			t.Errorf("Download(%s) error = %v", r.Job.URL, r.Error)
		}
	}

	for _, job := range jobs {
		if _, err := os.Stat(job.DestPath); os.IsNotExist(err) {
			t.Errorf("file %s was not created", job.DestPath)
		}
	}
}

func TestDownloader_Download_CreatesSubdirectories(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("content"))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	dl := NewDownloader(1, cacheDir)
	destPath := filepath.Join(cacheDir, "A", "AU", "AUTHOR", "Dist-1.0.tar.gz")

	jobs := []Job{{
		URL:      server.URL + "/dist.tar.gz",
		DestPath: destPath,
		Source:   "cpan",
	}}

	// Act
	results := dl.Download(jobs)

	// Assert
	if results[0].Error != nil {
		t.Errorf("Download() error = %v", results[0].Error)
	}
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("file was not created with subdirectories")
	}
}

func TestDownloader_CachePath(t *testing.T) {
	dl := NewDownloader(1, "/home/user/.yacm/cache")

	got := dl.CachePath("A/AU/AUTHOR/Dist-1.0.tar.gz")
	want := "/home/user/.yacm/cache/A/AU/AUTHOR/Dist-1.0.tar.gz"

	if got != want {
		t.Errorf("CachePath() = %q, want %q", got, want)
	}
}
