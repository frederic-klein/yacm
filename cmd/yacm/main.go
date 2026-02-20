package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/frederic-klein/yacm/internal/cpanfile"
	"github.com/frederic-klein/yacm/internal/dist"
	"github.com/frederic-klein/yacm/internal/downloader"
	"github.com/frederic-klein/yacm/internal/index"
	"github.com/frederic-klein/yacm/internal/resolver"
	"github.com/frederic-klein/yacm/internal/snapshot"
)

var (
	cpanfilePath string
	snapshotPath string
	workers      int
	mirror       string
	backpanDir   string
	verbose      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "yacm",
		Short: "Yet Another CPAN Manager - generates cpanfile.snapshot files",
		Long:  "YACM resolves Perl module dependencies from CPAN and BackPAN, generating snapshot files compatible with Carton and Carmel.",
	}

	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Generate cpanfile.snapshot from cpanfile",
		RunE:  runSnapshot,
	}

	snapshotCmd.Flags().StringVarP(&cpanfilePath, "cpanfile", "f", "./cpanfile", "Input cpanfile path")
	snapshotCmd.Flags().StringVarP(&snapshotPath, "snapshot", "s", "./cpanfile.snapshot", "Output snapshot path")
	snapshotCmd.Flags().IntVarP(&workers, "workers", "w", 5, "Parallel download workers")
	snapshotCmd.Flags().StringVarP(&mirror, "mirror", "m", "https://cpan.metacpan.org", "CPAN mirror URL")
	snapshotCmd.Flags().StringVar(&backpanDir, "backpan-dir", "./backpan-modules", "BackPAN modules directory")
	snapshotCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	rootCmd.AddCommand(snapshotCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runSnapshot(cmd *cobra.Command, args []string) error {
	log := func(format string, args ...interface{}) {
		if verbose {
			fmt.Printf(format+"\n", args...)
		}
	}

	// Parse cpanfile
	log("Parsing cpanfile: %s", cpanfilePath)
	parser := cpanfile.NewParser()
	parseResult, err := parser.Parse(cpanfilePath)
	if err != nil {
		return fmt.Errorf("parsing cpanfile: %w", err)
	}

	// Collect all requirements (runtime + test + build)
	var allReqs []dist.VersionReq
	for phase, reqs := range parseResult.Requirements {
		log("Found %d requirements for phase: %s", len(reqs), phase)
		allReqs = append(allReqs, reqs...)
	}

	if len(allReqs) == 0 {
		return fmt.Errorf("no requirements found in cpanfile")
	}

	// Setup cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".yacm", "cache")

	// Initialize CPAN index
	log("Loading CPAN index from %s", mirror)
	cpanIdx := index.NewCPANIndex(mirror, cacheDir)
	if err := cpanIdx.Load(); err != nil {
		return fmt.Errorf("loading CPAN index: %w", err)
	}

	// Initialize BackPAN index
	backpan := index.NewBackPANIndex(backpanDir)
	if err := backpan.EnsureDir(); err != nil {
		return fmt.Errorf("creating backpan directory: %w", err)
	}

	// Initialize downloader
	dl := downloader.NewDownloader(workers, cacheDir)

	// Resolve dependencies
	log("Resolving dependencies...")
	res := resolver.NewResolver(cpanIdx, backpan, dl, verbose)
	dists, err := res.Resolve(allReqs)
	if err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}

	log("Resolved %d distributions", len(dists))

	// Deduplicate distributions by pathname
	seen := make(map[string]bool)
	var uniqueDists []*dist.Dist
	for _, d := range dists {
		if !seen[d.Pathname] {
			seen[d.Pathname] = true
			uniqueDists = append(uniqueDists, d)
		}
	}

	// Write snapshot
	log("Writing snapshot: %s", snapshotPath)
	outFile, err := os.Create(snapshotPath)
	if err != nil {
		return fmt.Errorf("creating snapshot file: %w", err)
	}
	defer outFile.Close()

	emitter := snapshot.NewEmitter(outFile)
	if err := emitter.Emit(uniqueDists); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}

	fmt.Printf("Generated %s with %d distributions\n", snapshotPath, len(uniqueDists))
	return nil
}
