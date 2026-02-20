package cpanfile

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/frederic-klein/yacm/internal/dist"
)

// Parser parses cpanfile DSL.
type Parser struct{}

// NewParser creates a new cpanfile parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseResult contains parsed requirements grouped by phase.
type ParseResult struct {
	Requirements map[dist.Phase][]dist.VersionReq
}

// NewParseResult creates an empty parse result.
func NewParseResult() *ParseResult {
	return &ParseResult{
		Requirements: make(map[dist.Phase][]dist.VersionReq),
	}
}

var (
	requiresRe = regexp.MustCompile(`^\s*requires\s+['"]([^'"]+)['"](?:\s*,\s*['"]([^'"]+)['"])?`)
	onBlockRe  = regexp.MustCompile(`^\s*on\s+['"](\w+)['"]\s*=>\s*sub\s*\{`)
	closeRe    = regexp.MustCompile(`^\s*\}`)
)

// Parse parses a cpanfile and returns requirements by phase.
func (p *Parser) Parse(path string) (*ParseResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening cpanfile: %w", err)
	}
	defer file.Close()

	result := NewParseResult()
	currentPhase := dist.PhaseRuntime
	inBlock := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip comments and empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for on 'phase' => sub { block
		if matches := onBlockRe.FindStringSubmatch(line); matches != nil {
			currentPhase = parsePhase(matches[1])
			inBlock = true
			continue
		}

		// Check for closing brace
		if inBlock && closeRe.MatchString(line) {
			currentPhase = dist.PhaseRuntime
			inBlock = false
			continue
		}

		// Check for requires statement
		if matches := requiresRe.FindStringSubmatch(line); matches != nil {
			module := matches[1]
			version := "0"
			if matches[2] != "" {
				version = matches[2]
			}
			result.Requirements[currentPhase] = append(result.Requirements[currentPhase], dist.VersionReq{
				Module:  module,
				Version: version,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading cpanfile: %w", err)
	}

	return result, nil
}

func parsePhase(s string) dist.Phase {
	switch strings.ToLower(s) {
	case "test":
		return dist.PhaseTest
	case "develop":
		return dist.PhaseDevelop
	case "build":
		return dist.PhaseBuild
	default:
		return dist.PhaseRuntime
	}
}
