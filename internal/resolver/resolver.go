package resolver

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/frederic-klein/yacm/internal/dist"
	"github.com/frederic-klein/yacm/internal/downloader"
	"github.com/frederic-klein/yacm/internal/extractor"
	"github.com/frederic-klein/yacm/internal/index"
)

// Resolver resolves module dependencies recursively.
type Resolver struct {
	cpanIndex   *index.CPANIndex
	backpan     *index.BackPANIndex
	downloader  *downloader.Downloader
	extractor   *extractor.Extractor
	resolved    map[string]*dist.Dist
	resolving   map[string]bool
	verbose     bool
	logFn       func(string, ...interface{})
}

// NewResolver creates a new dependency resolver.
// If dockerImage is non-empty, configure steps run inside that Docker container.
func NewResolver(cpan *index.CPANIndex, backpan *index.BackPANIndex, dl *downloader.Downloader, verbose bool, dockerImage string) *Resolver {
	var ext *extractor.Extractor
	if dockerImage != "" {
		ext = extractor.NewDockerExtractor(dockerImage)
	} else {
		ext = extractor.NewExtractor()
	}

	return &Resolver{
		cpanIndex:  cpan,
		backpan:    backpan,
		downloader: dl,
		extractor:  ext,
		resolved:   make(map[string]*dist.Dist),
		resolving:  make(map[string]bool),
		verbose:    verbose,
		logFn: func(format string, args ...interface{}) {
			if verbose {
				fmt.Printf(format+"\n", args...)
			}
		},
	}
}

// Resolve resolves all dependencies for the given requirements.
func (r *Resolver) Resolve(reqs []dist.VersionReq) ([]*dist.Dist, error) {
	for _, req := range reqs {
		if err := r.resolveOne(req.Module, req.Version); err != nil {
			return nil, err
		}
	}

	dists := make([]*dist.Dist, 0, len(r.resolved))
	for _, d := range r.resolved {
		dists = append(dists, d)
	}
	return dists, nil
}

func (r *Resolver) resolveOne(module, version string) error {
	// Skip perl core modules
	if isCore(module) {
		return nil
	}

	// Check if already resolved with compatible version
	if d, ok := r.resolved[module]; ok {
		if satisfies(d.Provides[module], version) {
			return nil
		}
	}

	// Detect circular dependency
	if r.resolving[module] {
		r.logFn("Skipping circular dependency: %s", module)
		return nil
	}
	r.resolving[module] = true
	defer func() { delete(r.resolving, module) }()

	r.logFn("Resolving: %s %s", module, version)

	// Try CPAN first
	entry, found := r.cpanIndex.Lookup(module)
	var downloadURL, pathname, source string

	if found && satisfies(entry.Version, version) {
		pathname = entry.Pathname
		downloadURL = fmt.Sprintf("%s/authors/id/%s", r.cpanIndex.Mirror(), pathname)
		source = "cpan"
		r.logFn("  Found on CPAN: %s", pathname)
	} else {
		// Fallback to BackPAN
		r.logFn("  Trying BackPAN for %s %s", module, version)
		result, err := r.backpan.Lookup(module, version)
		if err != nil {
			return fmt.Errorf("resolving %s: %w", module, err)
		}
		downloadURL = result.DownloadURL
		pathname = extractPathname(downloadURL)
		source = "backpan"
		r.logFn("  Found on BackPAN: %s", pathname)
	}

	// Download the tarball
	var destPath string
	if source == "cpan" {
		destPath = r.downloader.CachePath(pathname)
	} else {
		destPath = r.backpan.LocalPath(downloadURL)
	}

	jobs := []downloader.Job{{
		URL:      downloadURL,
		DestPath: destPath,
		Source:   source,
	}}
	results := r.downloader.Download(jobs)
	if results[0].Error != nil {
		return fmt.Errorf("downloading %s: %w", module, results[0].Error)
	}

	// Extract META (with configure to resolve dynamic prerequisites)
	meta, err := r.extractor.ExtractWithConfigure(destPath)
	if err != nil {
		r.logFn("  Warning: %v, using minimal metadata", err)
		meta = &extractor.MetaFile{
			Name:         extractor.FlexVersion(distNameFromPath(pathname)),
			Provides:     map[string]extractor.ProvidesEntry{module: {Version: extractor.FlexVersion(version)}},
			Requirements: map[string]string{},
		}
	}

	// Create Dist
	d := &dist.Dist{
		Name:         distNameFromPath(pathname),
		Pathname:     pathname,
		Provides:     make(map[string]string),
		Requirements: meta.Requirements,
		Source:       source,
	}

	// Populate provides
	for mod, entry := range meta.Provides {
		d.Provides[mod] = string(entry.Version)
	}
	// Ensure the main module is in provides
	if _, ok := d.Provides[module]; !ok {
		d.Provides[module] = string(meta.Version)
	}

	// Mark as resolved (before recursing to handle circular deps)
	r.resolved[module] = d
	// Also mark by all provided modules
	for mod := range d.Provides {
		if _, exists := r.resolved[mod]; !exists {
			r.resolved[mod] = d
		}
	}

	// Resolve dependencies
	for depMod, depVer := range d.Requirements {
		if err := r.resolveOne(depMod, depVer); err != nil {
			return err
		}
	}

	return nil
}

func extractPathname(url string) string {
	// Extract pathname from URL like https://cpan.metacpan.org/authors/id/A/AU/AUTHOR/Dist.tar.gz
	idx := strings.Index(url, "/authors/id/")
	if idx != -1 {
		return url[idx+len("/authors/id/"):]
	}
	// Fallback: just return the filename
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

func distNameFromPath(pathname string) string {
	// A/AU/AUTHOR/Dist-Name-1.23.tar.gz -> Dist-Name-1.23
	base := filepath.Base(pathname)
	base = strings.TrimSuffix(base, ".tar.gz")
	base = strings.TrimSuffix(base, ".tgz")
	return base
}

var coreModules = map[string]bool{
	// Pragmas
	"perl": true, "strict": true, "warnings": true, "utf8": true,
	"base": true, "parent": true, "constant": true, "overload": true,
	"lib": true, "vars": true, "integer": true, "bytes": true,
	"feature": true, "if": true, "mro": true, "re": true,
	"locale": true, "open": true, "subs": true, "fields": true,
	"bignum": true, "bigint": true, "bigrat": true,

	// Core modules - A-C
	"AnyDBM_File": true, "AutoLoader": true, "AutoSplit": true,
	"B": true, "B::Deparse": true, "Benchmark": true,
	"Carp": true, "Carp::Heavy": true, "Class::Struct": true,
	"Config": true, "Config::Extensions": true, "Cwd": true,

	// Core modules - D-E
	"DB": true, "DBM_Filter": true, "Data::Dumper": true, "Devel::Peek": true,
	"Devel::SelfStubber": true, "Digest": true, "Digest::MD5": true,
	"DirHandle": true, "Dumpvalue": true, "DynaLoader": true,
	"Encode": true, "Encode::Alias": true, "Encode::Config": true,
	"Encode::Encoding": true, "Encode::Guess": true, "Encode::MIME::Header": true,
	"English": true, "Env": true, "Errno": true, "Exporter": true,
	"Exporter::Heavy": true, "ExtUtils::Constant": true,
	"ExtUtils::Embed": true, "ExtUtils::Install": true,
	"ExtUtils::Installed": true, "ExtUtils::Liblist": true,
	"ExtUtils::MM": true, "ExtUtils::MM_Any": true, "ExtUtils::MM_Unix": true,
	"ExtUtils::MY": true, "ExtUtils::Manifest": true, "ExtUtils::Miniperl": true,
	"ExtUtils::Mkbootstrap": true, "ExtUtils::Mksymlists": true,
	"ExtUtils::Packlist": true, "ExtUtils::testlib": true,

	// Core modules - F
	"Fcntl": true, "File::Basename": true, "File::Compare": true,
	"File::Copy": true, "File::DosGlob": true, "File::Find": true,
	"File::Glob": true, "File::Path": true, "File::Spec": true,
	"File::Spec::Functions": true, "File::Spec::Unix": true,
	"File::Stat": true, "File::stat": true, "File::Temp": true, "FileCache": true, "FileHandle": true,
	"Filter::Simple": true, "Filter::Util::Call": true, "FindBin": true,

	// Core modules - G-I
	"GDBM_File": true, "Getopt::Long": true, "Getopt::Std": true,
	"Hash::Util": true, "Hash::Util::FieldHash": true,
	"I18N::Collate": true, "I18N::LangTags": true, "I18N::Langinfo": true,
	"IO": true, "IO::Dir": true, "IO::File": true, "IO::Handle": true,
	"IO::Pipe": true, "IO::Poll": true, "IO::Seekable": true,
	"IO::Select": true, "IO::Socket": true, "IO::Socket::INET": true,
	"IO::Socket::UNIX": true, "IPC::Cmd": true, "IPC::Msg": true,
	"IPC::Open2": true, "IPC::Open3": true, "IPC::Semaphore": true,
	"IPC::SharedMem": true, "IPC::SysV": true,

	// Core modules - L-M
	"List::Util": true, "List::Util::XS": true, "Locale::Maketext": true,
	"MIME::Base64": true, "MIME::QuotedPrint": true, "Math::BigFloat": true,
	"Math::BigInt": true, "Math::BigRat": true, "Math::Complex": true,
	"Math::Trig": true, "Memoize": true,

	// Core modules - N-O
	"NDBM_File": true, "Net::Cmd": true, "Net::Config": true,
	"Net::Domain": true, "Net::FTP": true, "Net::NNTP": true,
	"Net::Netrc": true, "Net::POP3": true, "Net::Ping": true,
	"Net::SMTP": true, "Net::Time": true, "Net::hostent": true,
	"Net::netent": true, "Net::protoent": true, "Net::servent": true,
	"O": true, "Opcode": true, "ODBM_File": true, "OS2::Process": true,

	// Core modules - P
	"PerlIO": true, "PerlIO::encoding": true, "PerlIO::scalar": true,
	"PerlIO::via": true, "PerlIO::via::QuotedPrint": true,
	"Pod::Checker": true, "Pod::Find": true, "Pod::Functions": true,
	"Pod::Html": true, "Pod::InputObjects": true, "Pod::Man": true,
	"Pod::ParseLink": true, "Pod::ParseUtils": true, "Pod::Parser": true,
	"Pod::Perldoc": true, "Pod::PlainText": true, "Pod::Select": true,
	"Pod::Simple": true, "Pod::Text": true, "Pod::Usage": true,
	"POSIX": true,

	// Core modules - S
	"SDBM_File": true, "Safe": true, "Scalar::Util": true,
	"Search::Dict": true, "SelectSaver": true, "SelfLoader": true,
	"Socket": true, "Storable": true, "Sub::Util": true, "Symbol": true,
	"Sys::Hostname": true, "Sys::Syslog": true,

	// Core modules - T
	"Term::ANSIColor": true, "Term::Cap": true, "Term::Complete": true,
	"Term::ReadLine": true, "Test": true, "Test::Builder": true,
	"Test::Builder::Module": true, "Test::Builder::Tester": true,
	"Test::Harness": true, "Test::More": true, "Test::Simple": true,
	"Text::Abbrev": true, "Text::Balanced": true, "Text::ParseWords": true,
	"Text::Tabs": true, "Text::Wrap": true,
	"Thread": true, "Thread::Queue": true, "Thread::Semaphore": true,
	"Tie::Array": true, "Tie::File": true, "Tie::Handle": true,
	"Tie::Hash": true, "Tie::Memoize": true, "Tie::RefHash": true,
	"Tie::Scalar": true, "Tie::StdHandle": true, "Tie::SubstrHash": true,
	"Time::HiRes": true, "Time::Local": true, "Time::Piece": true,
	"Time::Seconds": true, "Time::gmtime": true, "Time::localtime": true,
	"Time::tm": true,

	// Core modules - U-X
	"UNIVERSAL": true, "Unicode::Collate": true, "Unicode::Normalize": true,
	"Unicode::UCD": true, "User::grent": true, "User::pwent": true,
	"XSLoader": true,

	// Frequently bundled but often core-ish
	"version": true, "threads": true, "threads::shared": true,
	"encoding": true, "encoding::warnings": true,
}

func isCore(module string) bool {
	return coreModules[module]
}

var versionRe = regexp.MustCompile(`^v?(\d+(?:\.\d+)*)`)

func satisfies(have, want string) bool {
	if want == "" || want == "0" {
		return true
	}
	// undef means version is unknown - treat as satisfying any requirement
	if have == "undef" {
		return true
	}
	if have == "" {
		have = "0"
	}

	// Parse version constraints
	constraints := strings.Split(want, ",")
	for _, c := range constraints {
		c = strings.TrimSpace(c)
		if !satisfiesOne(have, c) {
			return false
		}
	}
	return true
}

func satisfiesOne(have, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" || want == "0" {
		return true
	}

	var op, wantVer string
	if strings.HasPrefix(want, ">=") {
		op = ">="
		wantVer = strings.TrimSpace(want[2:])
	} else if strings.HasPrefix(want, "<=") {
		op = "<="
		wantVer = strings.TrimSpace(want[2:])
	} else if strings.HasPrefix(want, "!=") {
		op = "!="
		wantVer = strings.TrimSpace(want[2:])
	} else if strings.HasPrefix(want, ">") {
		op = ">"
		wantVer = strings.TrimSpace(want[1:])
	} else if strings.HasPrefix(want, "<") {
		op = "<"
		wantVer = strings.TrimSpace(want[1:])
	} else if strings.HasPrefix(want, "==") {
		op = "=="
		wantVer = strings.TrimSpace(want[2:])
	} else {
		op = ">="
		wantVer = want
	}

	cmp := compareVersions(have, wantVer)
	switch op {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case "<":
		return cmp < 0
	case "==":
		return cmp == 0
	case "!=":
		return cmp != 0
	}
	return true
}

func compareVersions(a, b string) int {
	aParts := normalizeVersion(a)
	bParts := normalizeVersion(b)

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		aVal := 0
		bVal := 0
		if i < len(aParts) {
			aVal = aParts[i]
		}
		if i < len(bParts) {
			bVal = bParts[i]
		}
		if aVal < bVal {
			return -1
		}
		if aVal > bVal {
			return 1
		}
	}
	return 0
}

// normalizeVersion converts a Perl version string to a slice of integers.
// Handles both dotted (v3.18.0, 3.18.0) and decimal (3.007004) formats.
// Decimal format: 3.007004 -> [3, 7, 4] (groups of 3 digits in fractional part)
// Dotted format: 3.18.0 -> [3, 18, 0]
func normalizeVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return []int{0}
	}

	parts := strings.Split(v, ".")
	if len(parts) == 1 {
		// Just a number like "5"
		n, _ := strconv.Atoi(parts[0])
		return []int{n}
	}

	// Check if this is decimal format (fractional part has many digits, no dots after first)
	// e.g., 3.007004 vs 3.18.0
	if len(parts) == 2 && len(parts[1]) > 3 {
		// Decimal format - split fractional part into groups of 3
		major, _ := strconv.Atoi(parts[0])
		result := []int{major}

		frac := parts[1]
		for len(frac) > 0 {
			chunk := frac
			if len(chunk) > 3 {
				chunk = frac[:3]
				frac = frac[3:]
			} else {
				frac = ""
			}
			n, _ := strconv.Atoi(chunk)
			result = append(result, n)
		}
		return result
	}

	// Dotted format - parse each part as integer
	result := make([]int, len(parts))
	for i, p := range parts {
		result[i], _ = strconv.Atoi(p)
	}
	return result
}
