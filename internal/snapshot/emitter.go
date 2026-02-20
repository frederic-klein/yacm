package snapshot

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/frederic-klein/yacm/internal/dist"
)

const header = "# carton snapshot format: version 1.0\n"

// Emitter writes snapshot files in Carton v1.0 format.
type Emitter struct {
	w io.Writer
}

// NewEmitter creates a new snapshot emitter.
func NewEmitter(w io.Writer) *Emitter {
	return &Emitter{w: w}
}

// Emit writes distributions to the snapshot in Carton v1.0 format.
func (e *Emitter) Emit(dists []*dist.Dist) error {
	// Sort distributions alphabetically by name
	sorted := make([]*dist.Dist, len(dists))
	copy(sorted, dists)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	if _, err := fmt.Fprint(e.w, header); err != nil {
		return err
	}

	if _, err := fmt.Fprint(e.w, "DISTRIBUTIONS\n"); err != nil {
		return err
	}

	for _, d := range sorted {
		if err := e.emitDist(d); err != nil {
			return err
		}
	}

	return nil
}

func (e *Emitter) emitDist(d *dist.Dist) error {
	// Distribution name with 2-space indent
	if _, err := fmt.Fprintf(e.w, "  %s\n", d.Name); err != nil {
		return err
	}

	// Pathname with 4-space indent
	if _, err := fmt.Fprintf(e.w, "    pathname: %s\n", d.Pathname); err != nil {
		return err
	}

	// Provides section
	if len(d.Provides) > 0 {
		if _, err := fmt.Fprint(e.w, "    provides:\n"); err != nil {
			return err
		}

		modules := sortedKeys(d.Provides)
		for _, mod := range modules {
			ver := d.Provides[mod]
			if ver == "" {
				ver = "undef"
			}
			if _, err := fmt.Fprintf(e.w, "      %s %s\n", mod, ver); err != nil {
				return err
			}
		}
	}

	// Requirements section
	if len(d.Requirements) > 0 {
		if _, err := fmt.Fprint(e.w, "    requirements:\n"); err != nil {
			return err
		}

		modules := sortedKeys(d.Requirements)
		for _, mod := range modules {
			ver := d.Requirements[mod]
			// Normalize version requirement for snapshot
			ver = normalizeVersion(ver)
			if _, err := fmt.Fprintf(e.w, "      %s %s\n", mod, ver); err != nil {
				return err
			}
		}
	}

	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func normalizeVersion(v string) string {
	// Strip operators and ranges, keep just minimum version
	v = strings.TrimSpace(v)
	if v == "" {
		return "0"
	}

	// Handle ranges like ">= 1.0, < 2.0" - take first version
	if idx := strings.Index(v, ","); idx != -1 {
		v = strings.TrimSpace(v[:idx])
	}

	// Strip operators
	v = strings.TrimPrefix(v, ">=")
	v = strings.TrimPrefix(v, ">")
	v = strings.TrimPrefix(v, "==")
	v = strings.TrimPrefix(v, "=")
	v = strings.TrimSpace(v)

	if v == "" {
		return "0"
	}
	return v
}
