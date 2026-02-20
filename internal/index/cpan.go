package index

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/frederic-klein/yacm/internal/dist"
)

const (
	defaultIndexPath = "modules/02packages.details.txt.gz"
	cacheTTL         = 24 * time.Hour
)

// CPANIndex provides lookup for modules from 02packages.details.txt.
type CPANIndex struct {
	mirror    string
	cacheDir  string
	modules   map[string]dist.CPANIndex
	cacheFile string
}

// NewCPANIndex creates a new CPAN index.
func NewCPANIndex(mirror, cacheDir string) *CPANIndex {
	return &CPANIndex{
		mirror:    strings.TrimSuffix(mirror, "/"),
		cacheDir:  cacheDir,
		modules:   make(map[string]dist.CPANIndex),
		cacheFile: filepath.Join(cacheDir, "02packages.details.txt"),
	}
}

// Load downloads and parses the CPAN index.
func (idx *CPANIndex) Load() error {
	if err := os.MkdirAll(idx.cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	if idx.isCacheValid() {
		return idx.parseCache()
	}

	if err := idx.download(); err != nil {
		return err
	}

	return idx.parseCache()
}

func (idx *CPANIndex) isCacheValid() bool {
	info, err := os.Stat(idx.cacheFile)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < cacheTTL
}

func (idx *CPANIndex) download() error {
	url := fmt.Sprintf("%s/%s", idx.mirror, defaultIndexPath)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading index: HTTP %d", resp.StatusCode)
	}

	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("decompressing index: %w", err)
	}
	defer gzReader.Close()

	outFile, err := os.Create(idx.cacheFile)
	if err != nil {
		return fmt.Errorf("creating cache file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, gzReader); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}

	return nil
}

func (idx *CPANIndex) parseCache() error {
	file, err := os.Open(idx.cacheFile)
	if err != nil {
		return fmt.Errorf("opening cache file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inHeader := true

	for scanner.Scan() {
		line := scanner.Text()

		// Skip header until empty line
		if inHeader {
			if line == "" {
				inHeader = false
			}
			continue
		}

		// Parse: Module::Name \t version \t A/AU/AUTHOR/Dist.tar.gz
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		module := fields[0]
		version := fields[1]
		pathname := fields[2]

		// Keep "undef" as-is so satisfies() can handle it properly
		idx.modules[module] = dist.CPANIndex{
			Module:   module,
			Version:  version,
			Pathname: pathname,
		}
	}

	return scanner.Err()
}

// Lookup finds a module in the index.
func (idx *CPANIndex) Lookup(module string) (dist.CPANIndex, bool) {
	entry, ok := idx.modules[module]
	return entry, ok
}

// Mirror returns the configured mirror URL.
func (idx *CPANIndex) Mirror() string {
	return idx.mirror
}
