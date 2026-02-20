package dist

// Dist represents a CPAN distribution with its metadata.
type Dist struct {
	Name         string            // e.g., "Module-Name-1.23"
	Pathname     string            // e.g., "A/AU/AUTHOR/Module-Name-1.23.tar.gz"
	Provides     map[string]string // module -> version
	Requirements map[string]string // module -> version constraint
	Source       string            // "cpan" or "backpan"
}

// VersionReq represents a module version requirement.
type VersionReq struct {
	Module  string
	Version string // e.g., ">= 1.0, < 2.0"
}

// Phase represents a dependency phase (runtime, test, develop, etc).
type Phase string

const (
	PhaseRuntime Phase = "runtime"
	PhaseTest    Phase = "test"
	PhaseDevelop Phase = "develop"
	PhaseBuild   Phase = "build"
)

// CPANIndex represents a module entry from 02packages.details.txt.
type CPANIndex struct {
	Module   string
	Version  string
	Pathname string
}
