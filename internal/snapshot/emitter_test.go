package snapshot

import (
	"bytes"
	"testing"

	"github.com/frederic-klein/yacm/internal/dist"
)

func TestEmitter_Emit(t *testing.T) {
	tests := []struct {
		name  string
		dists []*dist.Dist
		want  string
	}{
		{
			name:  "empty",
			dists: []*dist.Dist{},
			want:  "# carton snapshot format: version 1.0\nDISTRIBUTIONS\n",
		},
		{
			name: "single dist",
			dists: []*dist.Dist{
				{
					Name:     "JSON-2.0",
					Pathname: "M/MA/MAKAMAKA/JSON-2.0.tar.gz",
					Provides: map[string]string{
						"JSON": "2.0",
					},
					Requirements: map[string]string{},
				},
			},
			want: `# carton snapshot format: version 1.0
DISTRIBUTIONS
  JSON-2.0
    pathname: M/MA/MAKAMAKA/JSON-2.0.tar.gz
    provides:
      JSON 2.0
`,
		},
		{
			name: "with requirements",
			dists: []*dist.Dist{
				{
					Name:     "Moo-2.0",
					Pathname: "H/HA/HAARG/Moo-2.0.tar.gz",
					Provides: map[string]string{
						"Moo": "2.0",
					},
					Requirements: map[string]string{
						"Class::Method::Modifiers": "1.10",
						"Role::Tiny":               "2.0",
					},
				},
			},
			want: `# carton snapshot format: version 1.0
DISTRIBUTIONS
  Moo-2.0
    pathname: H/HA/HAARG/Moo-2.0.tar.gz
    provides:
      Moo 2.0
    requirements:
      Class::Method::Modifiers 1.10
      Role::Tiny 2.0
`,
		},
		{
			name: "sorted output",
			dists: []*dist.Dist{
				{
					Name:         "Zebra-1.0",
					Pathname:     "Z/ZE/ZEBRA/Zebra-1.0.tar.gz",
					Provides:     map[string]string{"Zebra": "1.0"},
					Requirements: map[string]string{},
				},
				{
					Name:         "Alpha-1.0",
					Pathname:     "A/AL/ALPHA/Alpha-1.0.tar.gz",
					Provides:     map[string]string{"Alpha": "1.0"},
					Requirements: map[string]string{},
				},
			},
			want: `# carton snapshot format: version 1.0
DISTRIBUTIONS
  Alpha-1.0
    pathname: A/AL/ALPHA/Alpha-1.0.tar.gz
    provides:
      Alpha 1.0
  Zebra-1.0
    pathname: Z/ZE/ZEBRA/Zebra-1.0.tar.gz
    provides:
      Zebra 1.0
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			emitter := NewEmitter(&buf)
			if err := emitter.Emit(tt.dists); err != nil {
				t.Fatalf("Emit() error = %v", err)
			}
			got := buf.String()
			if got != tt.want {
				t.Errorf("Emit() =\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "0"},
		{"0", "0"},
		{"1.0", "1.0"},
		{">= 1.0", "1.0"},
		{">= 1.0, < 2.0", "1.0"},
		{"> 1.0", "1.0"},
		{"== 1.0", "1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeVersion(tt.input)
			if got != tt.want {
				t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
