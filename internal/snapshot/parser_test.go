package snapshot

import (
	"strings"
	"testing"
)

func TestParser_Parse(t *testing.T) {
	input := `# carton snapshot format: version 1.0
DISTRIBUTIONS
  JSON-2.0
    pathname: M/MA/MAKAMAKA/JSON-2.0.tar.gz
    provides:
      JSON 2.0
      JSON::PP 2.0
    requirements:
      perl 5.008
  Moo-2.0
    pathname: H/HA/HAARG/Moo-2.0.tar.gz
    provides:
      Moo 2.0
    requirements:
      Class::Method::Modifiers 1.10
      Role::Tiny 2.0
`

	parser := NewParser(strings.NewReader(input))
	dists, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(dists) != 2 {
		t.Fatalf("got %d dists, want 2", len(dists))
	}

	// Check first dist
	d1 := dists[0]
	if d1.Name != "JSON-2.0" {
		t.Errorf("dist 0 name = %q, want JSON-2.0", d1.Name)
	}
	if d1.Pathname != "M/MA/MAKAMAKA/JSON-2.0.tar.gz" {
		t.Errorf("dist 0 pathname = %q", d1.Pathname)
	}
	if len(d1.Provides) != 2 {
		t.Errorf("dist 0 provides = %d, want 2", len(d1.Provides))
	}
	if d1.Provides["JSON"] != "2.0" {
		t.Errorf("dist 0 provides JSON = %q, want 2.0", d1.Provides["JSON"])
	}
	if len(d1.Requirements) != 1 {
		t.Errorf("dist 0 requirements = %d, want 1", len(d1.Requirements))
	}

	// Check second dist
	d2 := dists[1]
	if d2.Name != "Moo-2.0" {
		t.Errorf("dist 1 name = %q, want Moo-2.0", d2.Name)
	}
	if len(d2.Requirements) != 2 {
		t.Errorf("dist 1 requirements = %d, want 2", len(d2.Requirements))
	}
	if d2.Requirements["Role::Tiny"] != "2.0" {
		t.Errorf("dist 1 requires Role::Tiny = %q, want 2.0", d2.Requirements["Role::Tiny"])
	}
}

func TestParser_RoundTrip(t *testing.T) {
	// Parse, emit, parse again - should get same result
	input := `# carton snapshot format: version 1.0
DISTRIBUTIONS
  Alpha-1.0
    pathname: A/AL/ALPHA/Alpha-1.0.tar.gz
    provides:
      Alpha 1.0
    requirements:
      Beta 0
  Beta-2.0
    pathname: B/BE/BETA/Beta-2.0.tar.gz
    provides:
      Beta 2.0
`

	parser := NewParser(strings.NewReader(input))
	dists, err := parser.Parse()
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	var buf strings.Builder
	emitter := NewEmitter(&buf)
	if err := emitter.Emit(dists); err != nil {
		t.Fatalf("Emit() error = %v", err)
	}

	output := buf.String()
	if output != input {
		t.Errorf("round trip failed:\ngot:\n%s\nwant:\n%s", output, input)
	}
}
