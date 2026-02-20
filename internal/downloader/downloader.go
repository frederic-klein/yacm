package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Job represents a download job.
type Job struct {
	URL      string
	DestPath string
	Source   string // "cpan" or "backpan"
}

// Result represents a download result.
type Result struct {
	Job   Job
	Error error
}

// Downloader handles parallel HTTP downloads.
type Downloader struct {
	workers  int
	cacheDir string
	client   *http.Client
}

// NewDownloader creates a new downloader with the specified number of workers.
func NewDownloader(workers int, cacheDir string) *Downloader {
	return &Downloader{
		workers:  workers,
		cacheDir: cacheDir,
		client:   &http.Client{},
	}
}

// Download downloads multiple files in parallel.
func (d *Downloader) Download(jobs []Job) []Result {
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		results := make([]Result, len(jobs))
		for i, job := range jobs {
			results[i] = Result{Job: job, Error: err}
		}
		return results
	}

	jobChan := make(chan Job, len(jobs))
	resultChan := make(chan Result, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < d.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobChan {
				err := d.downloadOne(job)
				resultChan <- Result{Job: job, Error: err}
			}
		}()
	}

	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]Result, 0, len(jobs))
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

func (d *Downloader) downloadOne(job Job) error {
	// Check if already cached
	if _, err := os.Stat(job.DestPath); err == nil {
		return nil
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(job.DestPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	resp, err := d.client.Get(job.URL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", job.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", job.URL, resp.StatusCode)
	}

	// Write to temp file first, then rename
	tmpPath := job.DestPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing file: %w", err)
	}

	if err := os.Rename(tmpPath, job.DestPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming file: %w", err)
	}

	return nil
}

// CacheDir returns the cache directory.
func (d *Downloader) CacheDir() string {
	return d.cacheDir
}

// CachePath returns the cache path for a CPAN distribution.
func (d *Downloader) CachePath(pathname string) string {
	return filepath.Join(d.cacheDir, pathname)
}
