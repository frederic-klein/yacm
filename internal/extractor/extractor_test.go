package extractor

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func createTestTarball(t *testing.T, files map[string]string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tarballPath := filepath.Join(tmpDir, "test.tar.gz")

	f, err := os.Create(tarballPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	return tarballPath
}

func TestExtractor_Extract_MetaJSON(t *testing.T) {
	// Arrange
	metaJSON := `{
		"name": "JSON",
		"version": "4.10",
		"provides": {
			"JSON": {"file": "lib/JSON.pm", "version": "4.10"},
			"JSON::PP": {"file": "lib/JSON/PP.pm", "version": "4.10"}
		},
		"prereqs": {
			"runtime": {
				"requires": {
					"perl": "5.006",
					"Scalar::Util": "0"
				}
			}
		}
	}`

	tarballPath := createTestTarball(t, map[string]string{
		"JSON-4.10/META.json": metaJSON,
	})

	ext := NewExtractor()

	// Act
	meta, err := ext.Extract(tarballPath)

	// Assert
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if meta.Name != "JSON" {
		t.Errorf("Name = %q, want JSON", meta.Name)
	}
	if string(meta.Version) != "4.10" {
		t.Errorf("Version = %q, want 4.10", meta.Version)
	}
	if len(meta.Provides) != 2 {
		t.Errorf("Provides count = %d, want 2", len(meta.Provides))
	}
	if string(meta.Provides["JSON"].Version) != "4.10" {
		t.Errorf("Provides[JSON] = %q, want 4.10", meta.Provides["JSON"].Version)
	}
	if meta.Requirements["perl"] != "5.006" {
		t.Errorf("Requirements[perl] = %q, want 5.006", meta.Requirements["perl"])
	}
}

func TestExtractor_Extract_MetaYML(t *testing.T) {
	// Arrange
	// Note: YAML parses numeric-looking values as numbers, so "1.10" becomes 1.1
	metaYML := `---
name: Moo
version: '2.005005'
provides:
  Moo:
    file: lib/Moo.pm
    version: '2.005005'
prereqs:
  runtime:
    requires:
      perl: '5.006'
      Class::Method::Modifiers: '1.10'
`

	tarballPath := createTestTarball(t, map[string]string{
		"Moo-2.005005/META.yml": metaYML,
	})

	ext := NewExtractor()

	// Act
	meta, err := ext.Extract(tarballPath)

	// Assert
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if meta.Name != "Moo" {
		t.Errorf("Name = %q, want Moo", meta.Name)
	}
	if string(meta.Version) != "2.005005" {
		t.Errorf("Version = %q, want 2.005005", meta.Version)
	}
	if meta.Requirements["Class::Method::Modifiers"] != "1.10" {
		t.Errorf("Requirements[Class::Method::Modifiers] = %q, want 1.10", meta.Requirements["Class::Method::Modifiers"])
	}
}

func TestExtractor_Extract_PrefersJSON(t *testing.T) {
	// Arrange: Both META.json and META.yml present
	metaJSON := `{"name": "FromJSON", "version": "1.0"}`
	metaYML := `---
name: FromYAML
version: 2.0
`

	tarballPath := createTestTarball(t, map[string]string{
		"Dist-1.0/META.json": metaJSON,
		"Dist-1.0/META.yml":  metaYML,
	})

	ext := NewExtractor()

	// Act
	meta, err := ext.Extract(tarballPath)

	// Assert
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if meta.Name != "FromJSON" {
		t.Errorf("Name = %q, want FromJSON (should prefer META.json)", meta.Name)
	}
}

func TestExtractor_Extract_NoMeta(t *testing.T) {
	// Arrange
	tarballPath := createTestTarball(t, map[string]string{
		"Dist-1.0/lib/Dist.pm": "package Dist; 1;",
	})

	ext := NewExtractor()

	// Act
	_, err := ext.Extract(tarballPath)

	// Assert
	if err == nil {
		t.Error("Extract() should return error when no META file found")
	}
}

func TestExtractor_Extract_NestedMetaIgnored(t *testing.T) {
	// Arrange: META file nested too deep should be ignored
	metaJSON := `{"name": "Nested", "version": "1.0"}`

	tarballPath := createTestTarball(t, map[string]string{
		"Dist-1.0/subdir/META.json": metaJSON,
	})

	ext := NewExtractor()

	// Act
	_, err := ext.Extract(tarballPath)

	// Assert
	if err == nil {
		t.Error("Extract() should return error when META is nested too deep")
	}
}

func TestVersionString(t *testing.T) {
	tests := []struct {
		input interface{}
		want  string
	}{
		{"1.0", "1.0"},
		{1.5, "1.5"},
		{2, "2"},
		{nil, "0"},
		{true, "0"},
	}

	for _, tt := range tests {
		got := versionString(tt.input)
		if got != tt.want {
			t.Errorf("versionString(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractor_Extract_XAlienfile(t *testing.T) {
	// Arrange: Alien module with x_alienfile requirements
	metaJSON := `{
		"name": "Alien-Libxml2",
		"version": "0.20",
		"prereqs": {
			"runtime": {
				"requires": {
					"Alien::Build": "0.112"
				}
			}
		},
		"x_alienfile": {
			"requires": {
				"share": {
					"Mozilla::CA": "0",
					"IO::Socket::SSL": "1.56",
					"Net::SSLeay": "1.49"
				},
				"system": {
					"PkgConfig": "0.14026"
				}
			}
		}
	}`

	tarballPath := createTestTarball(t, map[string]string{
		"Alien-Libxml2-0.20/META.json": metaJSON,
	})

	ext := NewExtractor()

	// Act
	meta, err := ext.Extract(tarballPath)

	// Assert
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	// Should include regular prereqs
	if meta.Requirements["Alien::Build"] != "0.112" {
		t.Errorf("Requirements[Alien::Build] = %q, want 0.112", meta.Requirements["Alien::Build"])
	}

	// Should include x_alienfile share requirements
	if meta.Requirements["Mozilla::CA"] != "0" {
		t.Errorf("Requirements[Mozilla::CA] = %q, want 0", meta.Requirements["Mozilla::CA"])
	}
	if meta.Requirements["IO::Socket::SSL"] != "1.56" {
		t.Errorf("Requirements[IO::Socket::SSL] = %q, want 1.56", meta.Requirements["IO::Socket::SSL"])
	}

	// Should include x_alienfile system requirements
	if meta.Requirements["PkgConfig"] != "0.14026" {
		t.Errorf("Requirements[PkgConfig] = %q, want 0.14026", meta.Requirements["PkgConfig"])
	}
}

func TestExtractor_ExtractWithConfigure_PrefersMYMETA(t *testing.T) {
	// This test verifies that ExtractWithConfigure prefers MYMETA.json over META.json
	// when MYMETA.json exists (simulating running perl Makefile.PL)

	// Arrange: META.json with static prereqs, MYMETA.json with dynamic prereqs
	metaJSON := `{
		"name": "Test-Dist",
		"version": "1.0",
		"prereqs": {
			"runtime": {
				"requires": {
					"Some::Module": "1.0"
				}
			}
		}
	}`

	mymetaJSON := `{
		"name": "Test-Dist",
		"version": "1.0",
		"prereqs": {
			"runtime": {
				"requires": {
					"Some::Module": "1.0",
					"Dynamic::Dep": "2.0"
				}
			}
		}
	}`

	tarballPath := createTestTarball(t, map[string]string{
		"Test-Dist-1.0/META.json":   metaJSON,
		"Test-Dist-1.0/MYMETA.json": mymetaJSON,
	})

	ext := NewExtractor()

	// Act - use ExtractWithConfigure which should prefer MYMETA.json
	meta, err := ext.ExtractWithConfigure(tarballPath)

	// Assert
	if err != nil {
		t.Fatalf("ExtractWithConfigure() error = %v", err)
	}

	// Should include the dynamic prereq from MYMETA.json
	if meta.Requirements["Dynamic::Dep"] != "2.0" {
		t.Errorf("Requirements[Dynamic::Dep] = %q, want 2.0 (from MYMETA.json)", meta.Requirements["Dynamic::Dep"])
	}
}
