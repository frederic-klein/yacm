package extractor

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FlexVersion handles JSON/YAML values that can be string or number.
type FlexVersion string

func (v *FlexVersion) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*v = FlexVersion(s)
		return nil
	}
	// Try number
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		*v = FlexVersion(fmt.Sprintf("%g", f))
		return nil
	}
	*v = "0"
	return nil
}

func (v *FlexVersion) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err == nil {
		*v = FlexVersion(s)
		return nil
	}
	var f float64
	if err := node.Decode(&f); err == nil {
		*v = FlexVersion(fmt.Sprintf("%g", f))
		return nil
	}
	*v = "0"
	return nil
}

// XAlienfileRequires represents the requirements section of x_alienfile.
type XAlienfileRequires struct {
	Share  map[string]interface{} `json:"share" yaml:"share"`
	System map[string]interface{} `json:"system" yaml:"system"`
}

// XAlienfile represents the x_alienfile section in META files.
type XAlienfile struct {
	Requires XAlienfileRequires `json:"requires" yaml:"requires"`
}

// MetaFile represents the content of META.json or META.yml.
type MetaFile struct {
	Name         FlexVersion                       `json:"name" yaml:"name"`
	Version      FlexVersion                       `json:"version" yaml:"version"`
	Provides     map[string]ProvidesEntry          `json:"provides" yaml:"provides"`
	Prereqs      map[string]map[string]interface{} `json:"prereqs" yaml:"prereqs"`
	Requirements map[string]string                 `json:"-" yaml:"-"` // Flattened requirements
	XAlienfile   XAlienfile                        `json:"x_alienfile" yaml:"x_alienfile"`

	// Old META 1.x format fields
	Requires          map[string]interface{} `json:"requires" yaml:"requires"`
	BuildRequires     map[string]interface{} `json:"build_requires" yaml:"build_requires"`
	ConfigureRequires map[string]interface{} `json:"configure_requires" yaml:"configure_requires"`
}

// ProvidesEntry represents a module provided by the distribution.
type ProvidesEntry struct {
	File    string      `json:"file" yaml:"file"`
	Version FlexVersion `json:"version" yaml:"version"`
}

// Extractor extracts META files from CPAN tarballs.
type Extractor struct {
	dockerImage string // If set, run configure inside this Docker image
}

// NewExtractor creates a new extractor that runs configure on the host.
func NewExtractor() *Extractor {
	return &Extractor{}
}

// NewDockerExtractor creates an extractor that runs configure inside Docker.
// This ensures consistent dynamic prereq resolution regardless of host system.
func NewDockerExtractor(image string) *Extractor {
	return &Extractor{dockerImage: image}
}

// Extract reads META.json or META.yml from a tarball (without running configure).
func (e *Extractor) Extract(tarballPath string) (*MetaFile, error) {
	return e.extractMeta(tarballPath, false)
}

// ExtractWithConfigure extracts and runs perl Makefile.PL to get MYMETA.json
// with resolved dynamic prerequisites. Falls back to META.json if configure fails.
func (e *Extractor) ExtractWithConfigure(tarballPath string) (*MetaFile, error) {
	return e.extractMeta(tarballPath, true)
}

// extractMeta reads META files from a tarball.
// If withConfigure is true, it prefers MYMETA.json and will run configure if needed.
func (e *Extractor) extractMeta(tarballPath string, withConfigure bool) (*MetaFile, error) {
	file, err := os.Open(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("opening tarball: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("decompressing tarball: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	var metaJSON, metaYML, mymetaJSON, mymetaYML []byte
	var hasMakefilePL, hasBuildPL bool

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tarball: %w", err)
		}

		name := filepath.Base(header.Name)
		// Only look at top-level files (one directory deep)
		parts := strings.Split(header.Name, "/")
		if len(parts) != 2 {
			continue
		}

		switch name {
		case "META.json":
			metaJSON, err = io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("reading META.json: %w", err)
			}
		case "META.yml":
			metaYML, err = io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("reading META.yml: %w", err)
			}
		case "MYMETA.json":
			mymetaJSON, err = io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("reading MYMETA.json: %w", err)
			}
		case "MYMETA.yml":
			mymetaYML, err = io.ReadAll(tarReader)
			if err != nil {
				return nil, fmt.Errorf("reading MYMETA.yml: %w", err)
			}
		case "Makefile.PL":
			hasMakefilePL = true
		case "Build.PL":
			hasBuildPL = true
		}
	}

	// If withConfigure is true, prefer MYMETA files
	if withConfigure {
		// If we have MYMETA files in the tarball, use them
		if mymetaJSON != nil {
			return e.parseJSON(mymetaJSON)
		}
		if mymetaYML != nil {
			return e.parseYAML(mymetaYML)
		}

		// If we have a configure script, run it to generate MYMETA
		if hasMakefilePL || hasBuildPL {
			meta, err := e.runConfigure(tarballPath, hasMakefilePL)
			if err == nil {
				return meta, nil
			}
			// Fall back to META if configure fails
		}
	}

	// Fall back to META.json or META.yml
	if metaJSON != nil {
		return e.parseJSON(metaJSON)
	}
	if metaYML != nil {
		return e.parseYAML(metaYML)
	}

	return nil, fmt.Errorf("no META.json or META.yml found in tarball")
}

// runConfigure extracts tarball, runs configure, and parses MYMETA.json
func (e *Extractor) runConfigure(tarballPath string, hasMakefilePL bool) (*MetaFile, error) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "yacm-configure-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract tarball
	distDir, err := e.extractTarball(tarballPath, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("extracting tarball: %w", err)
	}

	// Determine configure script
	configScript := "Build.PL"
	if hasMakefilePL {
		configScript = "Makefile.PL"
	}

	// Run configure (in Docker or on host)
	var cmd *exec.Cmd
	if e.dockerImage != "" {
		// Run inside Docker container
		// Mount the dist directory and run perl Makefile.PL
		cmd = exec.Command("docker", "run", "--rm",
			"-v", distDir+":/work",
			"-w", "/work",
			e.dockerImage,
			"perl", configScript)
	} else {
		// Run on host
		cmd = exec.Command("perl", configScript)
		cmd.Dir = distDir
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running configure: %w", err)
	}

	// Read MYMETA.json or MYMETA.yml
	mymetaJSON := filepath.Join(distDir, "MYMETA.json")
	if data, err := os.ReadFile(mymetaJSON); err == nil {
		return e.parseJSON(data)
	}

	mymetaYML := filepath.Join(distDir, "MYMETA.yml")
	if data, err := os.ReadFile(mymetaYML); err == nil {
		return e.parseYAML(data)
	}

	return nil, fmt.Errorf("no MYMETA file generated")
}

// extractTarball extracts a tarball to destDir and returns the extracted directory path
func (e *Extractor) extractTarball(tarballPath, destDir string) (string, error) {
	file, err := os.Open(tarballPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var rootDir string

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// Get the root directory name from the first entry
		parts := strings.SplitN(header.Name, "/", 2)
		if rootDir == "" && len(parts) > 0 {
			rootDir = parts[0]
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return "", err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				return "", err
			}
			f.Close()
		}
	}

	return filepath.Join(destDir, rootDir), nil
}

func (e *Extractor) parseJSON(data []byte) (*MetaFile, error) {
	var meta MetaFile
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing META.json: %w", err)
	}
	e.flattenPrereqs(&meta)
	return &meta, nil
}

func (e *Extractor) parseYAML(data []byte) (*MetaFile, error) {
	var meta MetaFile
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing META.yml: %w", err)
	}
	e.flattenPrereqs(&meta)
	return &meta, nil
}

func (e *Extractor) flattenPrereqs(meta *MetaFile) {
	meta.Requirements = make(map[string]string)

	// Handle META 2.0 format (prereqs)
	// Include requires, recommends, and suggests to match Carmel behavior
	phases := []string{"runtime", "configure", "build"}
	depTypes := []string{"requires", "recommends", "suggests"}
	for _, phase := range phases {
		if phaseReqs, ok := meta.Prereqs[phase]; ok {
			for _, depType := range depTypes {
				if deps, ok := phaseReqs[depType]; ok {
					if reqMap, ok := deps.(map[string]interface{}); ok {
						for mod, ver := range reqMap {
							if meta.Requirements[mod] == "" {
								meta.Requirements[mod] = versionString(ver)
							}
						}
					}
				}
			}
		}
	}

	// Handle META 1.x format (requires, build_requires, configure_requires)
	for mod, ver := range meta.Requires {
		if meta.Requirements[mod] == "" {
			meta.Requirements[mod] = versionString(ver)
		}
	}
	for mod, ver := range meta.BuildRequires {
		if meta.Requirements[mod] == "" {
			meta.Requirements[mod] = versionString(ver)
		}
	}
	for mod, ver := range meta.ConfigureRequires {
		if meta.Requirements[mod] == "" {
			meta.Requirements[mod] = versionString(ver)
		}
	}

	// Handle x_alienfile requirements (for Alien:: modules)
	for mod, ver := range meta.XAlienfile.Requires.Share {
		if meta.Requirements[mod] == "" {
			meta.Requirements[mod] = versionString(ver)
		}
	}
	for mod, ver := range meta.XAlienfile.Requires.System {
		if meta.Requirements[mod] == "" {
			meta.Requirements[mod] = versionString(ver)
		}
	}
}

func versionString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%g", val)
	case int:
		return fmt.Sprintf("%d", val)
	default:
		return "0"
	}
}
