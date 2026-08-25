package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Required coverage percentages for critical subsystem packages
var criticalThresholds = map[string]float64{
	"github.com/RUSEGAL/ruseon-core/v2/pkg/logger":          90.0,
	"github.com/RUSEGAL/ruseon-core/v2/pkg/eventbus":        90.0,
	"github.com/RUSEGAL/ruseon-core/v2/pkg/registry":        90.0,
	"github.com/RUSEGAL/ruseon-core/v2/pkg/storage/localfs": 85.0,
	"github.com/RUSEGAL/ruseon-core/v2/pkg/auth":            80.0,
	"github.com/RUSEGAL/ruseon-core/v2/pkg/storage":         75.0,
	"github.com/RUSEGAL/ruseon-core/v2/pkg/config":          70.0,
	"github.com/RUSEGAL/ruseon-core/v2/internal/archive":    80.0,
	"github.com/RUSEGAL/ruseon-core/v2/internal/buffer":     70.0,
	"github.com/RUSEGAL/ruseon-core/v2/internal/backup":     70.0,
	"github.com/RUSEGAL/ruseon-core/v2/internal/stream":     65.0,
	"github.com/RUSEGAL/ruseon-core/v2/internal/grpc":       65.0,
	"github.com/RUSEGAL/ruseon-core/v2/internal/recorder":   65.0,
	"github.com/RUSEGAL/ruseon-core/v2/internal/mqtt":       80.0,
}

const globalMinimumThreshold = 50.0

type packageStats struct {
	totalStmts   int
	coveredStmts int
}

func parseCoverage(coverageFile string) (map[string]*packageStats, int, int, error) {
	cleanPath := filepath.Clean(coverageFile)
	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("error opening coverage profile %s: %w", cleanPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	pkgStats := make(map[string]*packageStats)
	var globalTotal, globalCovered int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		// Format: github.com/RUSEGAL/ruseon-core/v2/pkg/storage/storage.go:37.45,40.16 2 1
		colonIdx := strings.LastIndex(line, ":")
		if colonIdx == -1 {
			continue
		}

		filePath := line[:colonIdx]
		rest := line[colonIdx+1:]
		fields := strings.Fields(rest)
		if len(fields) < 3 {
			continue
		}

		stmts, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}

		// Skip generated protobuf files, swagger docs, and main command entrypoints
		if strings.Contains(filePath, ".pb.go") || strings.Contains(filePath, "_grpc.pb.go") ||
			strings.Contains(filePath, "/docs/") || strings.Contains(filePath, "cmd/") {
			continue
		}

		pkgPath := path.Dir(filePath)
		if _, exists := pkgStats[pkgPath]; !exists {
			pkgStats[pkgPath] = &packageStats{}
		}

		pkgStats[pkgPath].totalStmts += stmts
		globalTotal += stmts

		if count > 0 {
			pkgStats[pkgPath].coveredStmts += stmts
			globalCovered += stmts
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, 0, 0, fmt.Errorf("error scanning coverage profile: %w", err)
	}

	return pkgStats, globalTotal, globalCovered, nil
}

func main() {
	coverageFile := "coverage.out"
	if len(os.Args) > 1 {
		coverageFile = os.Args[1]
	}

	pkgStats, globalTotal, globalCovered, err := parseCoverage(coverageFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("=========================================================================================")
	fmt.Printf("%-60s | %-10s | %-10s | %-6s\n", "Package", "Coverage", "Required", "Status")
	fmt.Println("-----------------------------------------------------------------------------------------")

	var failedThresholds []string

	// Sort package names with preallocation
	pkgs := make([]string, 0, len(criticalThresholds))
	for p := range criticalThresholds {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	for _, pkg := range pkgs {
		req := criticalThresholds[pkg]
		stats, exists := pkgStats[pkg]
		var actual float64
		if exists && stats.totalStmts > 0 {
			actual = (float64(stats.coveredStmts) / float64(stats.totalStmts)) * 100.0
		}

		status := "PASS"
		if actual < req {
			status = "FAIL"
			failedThresholds = append(failedThresholds, fmt.Sprintf("%s: actual %.1f%% < required %.1f%%", pkg, actual, req))
		}

		fmt.Printf("%-60s | %8.1f%%  | %8.1f%%  | [%s]\n", pkg, actual, req, status)
	}

	fmt.Println("-----------------------------------------------------------------------------------------")
	var globalPct float64
	if globalTotal > 0 {
		globalPct = (float64(globalCovered) / float64(globalTotal)) * 100.0
	}
	globalStatus := "PASS"
	if globalPct < globalMinimumThreshold {
		globalStatus = "FAIL"
		failedThresholds = append(failedThresholds, fmt.Sprintf("Global coverage: actual %.1f%% < required %.1f%%", globalPct, globalMinimumThreshold))
	}
	fmt.Printf("%-60s | %8.1f%%  | %8.1f%%  | [%s]\n", "Total Non-Generated Code Coverage", globalPct, globalMinimumThreshold, globalStatus)
	fmt.Println("=========================================================================================")

	if len(failedThresholds) > 0 {
		fmt.Fprintf(os.Stderr, "\n[ERROR] Code coverage validation failed for %d threshold(s):\n", len(failedThresholds))
		for _, f := range failedThresholds {
			fmt.Fprintf(os.Stderr, "  - %s\n", f)
		}
		os.Exit(1)
	}

	fmt.Println("\n[SUCCESS] All critical package coverage thresholds satisfied!")
}
