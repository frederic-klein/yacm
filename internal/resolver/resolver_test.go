package resolver

import "testing"

func TestSatisfies(t *testing.T) {
	tests := []struct {
		have string
		want string
		ok   bool
	}{
		{"1.0", "", true},
		{"1.0", "0", true},
		{"1.0", "1.0", true},
		{"2.0", "1.0", true},
		{"0.5", "1.0", false},
		{"2.0", ">= 1.0", true},
		{"1.0", ">= 1.0", true},
		{"0.9", ">= 1.0", false},
		{"0.9", "< 1.0", true},
		{"1.0", "< 1.0", false},
		{"1.5", "> 1.0", true},
		{"1.0", "> 1.0", false},
		{"1.0", "<= 1.0", true},
		{"1.1", "<= 1.0", false},
		{"1.0", "== 1.0", true},
		{"1.1", "== 1.0", false},
		{"1.1", "!= 1.0", true},
		{"1.0", "!= 1.0", false},
		{"1.5", ">= 1.0, < 2.0", true},
		{"0.9", ">= 1.0, < 2.0", false},
		{"2.0", ">= 1.0, < 2.0", false},
		{"undef", "0", true},
		{"undef", "1.0", true},  // undef satisfies any version
		{"undef", ">= 2.0", true},
		{"", "0", true},
	}

	for _, tt := range tests {
		t.Run(tt.have+"_"+tt.want, func(t *testing.T) {
			got := satisfies(tt.have, tt.want)
			if got != tt.ok {
				t.Errorf("satisfies(%q, %q) = %v, want %v", tt.have, tt.want, got, tt.ok)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		{"1.0", "1.0", 0},
		{"1.0", "2.0", -1},
		{"2.0", "1.0", 1},
		{"1.10", "1.9", 1},
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"v1.0", "1.0", 0},
		{"1", "1.0", 0},
		{"1.0", "1", 0},
		{"1.001", "1.1", 0},
		// Perl decimal format tests
		{"3.18.0", "3.007004", 1},  // 3.18.0 > 3.7.4
		{"3.007004", "3.18.0", -1}, // 3.7.4 < 3.18.0
		{"3.007004", "3.007004", 0},
		{"0.080001", "0.08", 1},    // 0.80.1 > 0.8
		{"2.005005", "2.005", 1},   // 2.5.5 > 2.5
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.0", []int{1, 0}},
		{"3.18.0", []int{3, 18, 0}},
		{"3.007004", []int{3, 7, 4}},   // Decimal format
		{"0.080001", []int{0, 80, 1}},  // Decimal format
		{"2.005005", []int{2, 5, 5}},   // Decimal format
		{"v1.2.3", []int{1, 2, 3}},
		{"5", []int{5}},
		{"", []int{0}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeVersion(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("normalizeVersion(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("normalizeVersion(%q) = %v, want %v", tt.input, got, tt.want)
					return
				}
			}
		})
	}
}

func TestIsCore(t *testing.T) {
	cores := []string{"perl", "strict", "warnings", "Exporter", "Carp"}
	for _, mod := range cores {
		if !isCore(mod) {
			t.Errorf("isCore(%q) = false, want true", mod)
		}
	}

	nonCores := []string{"JSON", "Moo", "Moose", "DBI"}
	for _, mod := range nonCores {
		if isCore(mod) {
			t.Errorf("isCore(%q) = true, want false", mod)
		}
	}
}

func TestDistNameFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"M/MA/MAKAMAKA/JSON-2.0.tar.gz", "JSON-2.0"},
		{"H/HA/HAARG/Moo-2.005005.tar.gz", "Moo-2.005005"},
		{"S/SH/SHAY/Perl-Dist-1.23.tgz", "Perl-Dist-1.23"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := distNameFromPath(tt.path)
			if got != tt.want {
				t.Errorf("distNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
