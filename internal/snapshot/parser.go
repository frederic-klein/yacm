package snapshot

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/frederic-klein/yacm/internal/dist"
)

var (
	distNameRe   = regexp.MustCompile(`^  (\S+)$`)
	pathnameRe   = regexp.MustCompile(`^    pathname: (.+)$`)
	providesRe   = regexp.MustCompile(`^    provides:$`)
	requiresRe   = regexp.MustCompile(`^    requirements:$`)
	moduleVerRe  = regexp.MustCompile(`^      (\S+) (.+)$`)
)

// Parser reads snapshot files in Carton v1.0 format.
type Parser struct {
	r io.Reader
}

// NewParser creates a new snapshot parser.
func NewParser(r io.Reader) *Parser {
	return &Parser{r: r}
}

// Parse reads distributions from a snapshot file.
func (p *Parser) Parse() ([]*dist.Dist, error) {
	var dists []*dist.Dist
	var current *dist.Dist
	var inProvides, inRequirements bool

	scanner := bufio.NewScanner(p.r)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip header and DISTRIBUTIONS line
		if strings.HasPrefix(line, "#") || line == "DISTRIBUTIONS" {
			continue
		}

		// Empty line
		if line == "" {
			continue
		}

		// Distribution name (2-space indent)
		if matches := distNameRe.FindStringSubmatch(line); matches != nil {
			if current != nil {
				dists = append(dists, current)
			}
			current = &dist.Dist{
				Name:         matches[1],
				Provides:     make(map[string]string),
				Requirements: make(map[string]string),
			}
			inProvides = false
			inRequirements = false
			continue
		}

		if current == nil {
			continue
		}

		// Pathname
		if matches := pathnameRe.FindStringSubmatch(line); matches != nil {
			current.Pathname = matches[1]
			continue
		}

		// Section headers
		if providesRe.MatchString(line) {
			inProvides = true
			inRequirements = false
			continue
		}
		if requiresRe.MatchString(line) {
			inRequirements = true
			inProvides = false
			continue
		}

		// Module version entries (6-space indent)
		if matches := moduleVerRe.FindStringSubmatch(line); matches != nil {
			module := matches[1]
			version := matches[2]
			if inProvides {
				current.Provides[module] = version
			} else if inRequirements {
				current.Requirements[module] = version
			}
		}
	}

	// Don't forget the last distribution
	if current != nil {
		dists = append(dists, current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}

	return dists, nil
}
