package cpanfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/frederic-klein/yacm/internal/dist"
)

func TestParser_Parse(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantReqs map[dist.Phase][]dist.VersionReq
	}{
		{
			name:    "simple requires",
			content: `requires 'JSON';`,
			wantReqs: map[dist.Phase][]dist.VersionReq{
				dist.PhaseRuntime: {{Module: "JSON", Version: "0"}},
			},
		},
		{
			name:    "requires with version",
			content: `requires 'JSON', '2.0';`,
			wantReqs: map[dist.Phase][]dist.VersionReq{
				dist.PhaseRuntime: {{Module: "JSON", Version: "2.0"}},
			},
		},
		{
			name:    "requires with version constraint",
			content: `requires 'Moo', '>= 2.0, < 3.0';`,
			wantReqs: map[dist.Phase][]dist.VersionReq{
				dist.PhaseRuntime: {{Module: "Moo", Version: ">= 2.0, < 3.0"}},
			},
		},
		{
			name: "multiple requires",
			content: `requires 'JSON', '2.0';
requires 'Moo', '>= 2.0';`,
			wantReqs: map[dist.Phase][]dist.VersionReq{
				dist.PhaseRuntime: {
					{Module: "JSON", Version: "2.0"},
					{Module: "Moo", Version: ">= 2.0"},
				},
			},
		},
		{
			name: "on test block",
			content: `requires 'JSON';
on 'test' => sub {
    requires 'Test::More';
};`,
			wantReqs: map[dist.Phase][]dist.VersionReq{
				dist.PhaseRuntime: {{Module: "JSON", Version: "0"}},
				dist.PhaseTest:    {{Module: "Test::More", Version: "0"}},
			},
		},
		{
			name: "with comments",
			content: `# This is a comment
requires 'JSON';  # inline comment not supported but line works`,
			wantReqs: map[dist.Phase][]dist.VersionReq{
				dist.PhaseRuntime: {{Module: "JSON", Version: "0"}},
			},
		},
		{
			name: "double quotes",
			content: `requires "JSON", "2.0";`,
			wantReqs: map[dist.Phase][]dist.VersionReq{
				dist.PhaseRuntime: {{Module: "JSON", Version: "2.0"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cpanfilePath := filepath.Join(tmpDir, "cpanfile")
			if err := os.WriteFile(cpanfilePath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			parser := NewParser()
			result, err := parser.Parse(cpanfilePath)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			for phase, wantReqs := range tt.wantReqs {
				gotReqs := result.Requirements[phase]
				if len(gotReqs) != len(wantReqs) {
					t.Errorf("phase %s: got %d reqs, want %d", phase, len(gotReqs), len(wantReqs))
					continue
				}
				for i, want := range wantReqs {
					got := gotReqs[i]
					if got.Module != want.Module || got.Version != want.Version {
						t.Errorf("phase %s req %d: got %+v, want %+v", phase, i, got, want)
					}
				}
			}
		})
	}
}
