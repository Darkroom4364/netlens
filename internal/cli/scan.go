package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/Darkroom4364/netlens/internal/format"
	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/style"
	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	var (
		source       string
		msmID        int
		file         string
		method       string
		outputFormat string
		start        int64
		stop         int64
		maxAnonymous float64
		useCache     bool
		top          int
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run end-to-end tomography on real measurement data",
		Long: `Fetches traceroute measurements from RIPE Atlas or a local file,
infers the network topology, builds the routing matrix, runs identifiability
analysis, solves the inverse problem, and outputs per-link estimates.`,
		Example: `  netlens scan --source ripe --msm 1001 --cache
  netlens scan --source traceroute --file traces.json -m tikhonov --top 20
  netlens scan --source ripe --msm 1001 -f json > results.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxAnonymous < 0 || maxAnonymous > 1 {
				return fmt.Errorf("--max-anonymous must be in [0, 1] (got %.4f)", maxAnonymous)
			}

			measurements, err := loadMeasurements(source, msmID, file, start, stop, useCache)
			if err != nil {
				return err
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				fmt.Printf("Loaded %d measurements from %s\n", len(measurements), source)
			}

			// 2. Infer topology from traceroute hops
			opts := topology.InferOpts{
				MaxAnonymousFrac: maxAnonymous,
			}
			graph, pathSpecs, acceptedIdx, err := topology.InferFromMeasurements(measurements, opts)
			if err != nil {
				return fmt.Errorf("infer topology: %w", err)
			}

			// Use the accepted indices returned by InferFromMeasurements
			// to select the matching measurements.
			accepted := make([]tomo.PathMeasurement, len(acceptedIdx))
			for i, idx := range acceptedIdx {
				accepted[i] = measurements[idx]
			}

			if !quiet {
				fmt.Printf("Topology:       %d nodes, %d links\n", graph.NumNodes(), graph.NumLinks())
				fmt.Printf("Paths:          %d (of %d measurements)\n", len(pathSpecs), len(measurements))
			}

			// 3. Build routing matrix + Problem
			problem, err := tomo.BuildProblemFromMeasurements(graph, accepted, pathSpecs)
			if err != nil {
				return fmt.Errorf("build problem: %w", err)
			}

			// 4. Identifiability analysis (already computed in BuildProblem)
			q := problem.Quality
			if !quiet {
				fmt.Printf("Matrix rank:    %d / %d (identifiable: %.0f%%)\n",
					q.Rank, q.NumLinks, q.IdentifiableFrac*100)
				fmt.Printf("Condition:      %.2f\n", q.ConditionNumber)
			}

			// 5. Solve with selected method
			solver, err := getSolver(method)
			if err != nil {
				return err
			}
			sol, err := solver.Solve(cmd.Context(), problem)
			if err != nil {
				return fmt.Errorf("solve: %w", err)
			}

			warnNegativeDelays(cmd, sol, method)

			if !quiet {
				fmt.Printf("Solver:         %s\n", sol.Method)
				fmt.Printf("Duration:       %v\n", sol.Duration)
				fmt.Printf("Residual:       %.6f\n\n", sol.Residual)
			}

			// 6. Output results
			if !style.IsTTY && outputFormat == "table" {
				outputFormat = "json"
			}

			if outputFormat == "table" {
				printScanTable(problem, sol, top, quiet)
				return nil
			}

			formatter := format.Get(outputFormat)
			if formatter == nil {
				return fmt.Errorf("unknown format %q (expected \"json\", \"csv\", \"dot\", or \"table\")", outputFormat)
			}
			return formatter.Format(os.Stdout, problem, sol)
		},
	}

	cmd.Flags().StringVar(&source, "source", "", "Measurement source: \"ripe\" or \"traceroute\" (required)")
	cmd.Flags().IntVar(&msmID, "msm", 0, "RIPE Atlas measurement ID (required if --source=ripe)")
	cmd.Flags().StringVar(&file, "file", "", "Path to traceroute JSON file (required if --source=traceroute)")
	cmd.Flags().StringVarP(&method, "method", "m", "nnls", "Solver method: tsvd, tikhonov, nnls, admm, irl1, vardi, tomogravity, laplacian")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "table", "Output format: json, csv, dot, table")

	now := time.Now().Unix()
	oneHourAgo := now - 3600
	cmd.Flags().Int64Var(&start, "start", oneHourAgo, "UNIX timestamp for RIPE Atlas result window start")
	cmd.Flags().Int64Var(&stop, "stop", now, "UNIX timestamp for RIPE Atlas result window stop")
	cmd.Flags().Float64Var(&maxAnonymous, "max-anonymous", 0.3, "Max anonymous hop fraction before discarding path")
	cmd.Flags().BoolVar(&useCache, "cache", false, "Cache RIPE Atlas results locally (~/.cache/netlens/)")
	cmd.Flags().IntVar(&top, "top", 0, "Show only the N worst links (0 = show all)")

	_ = cmd.MarkFlagRequired("source")

	return cmd
}

// loadMeasurements fetches or reads traceroute measurements from the given source.
func loadMeasurements(source string, msmID int, file string, start, stop int64, useCache bool) ([]tomo.PathMeasurement, error) {
	var measurements []tomo.PathMeasurement
	var err error

	switch source {
	case "ripe":
		if msmID == 0 {
			return nil, fmt.Errorf("--msm is required when --source=ripe")
		}

		var cache *measure.Cache
		var cacheKey string
		if useCache {
			cache = measure.NewCache("")
			cacheKey = cache.Key(strconv.Itoa(msmID), strconv.FormatInt(start, 10), strconv.FormatInt(stop, 10))
		}

		if cache != nil && cache.Has(cacheKey) {
			raw, loadErr := cache.Load(cacheKey)
			if loadErr != nil {
				return nil, fmt.Errorf("load cache: %w", loadErr)
			}
			if err = json.Unmarshal(raw, &measurements); err != nil {
				return nil, fmt.Errorf("parse cached results: %w", err)
			}
			fmt.Println("Loaded results from cache")
		} else {
			apiKey := os.Getenv("RIPE_ATLAS_API_KEY")
			src := measure.NewRIPEAtlasSource(apiKey, "", nil)
			ctx := context.Background()
			measurements, err = src.FetchResults(ctx, msmID, start, stop)
			if err != nil {
				return nil, fmt.Errorf("fetch RIPE Atlas results: %w", err)
			}
			if cache != nil {
				raw, marshalErr := json.Marshal(measurements)
				if marshalErr == nil {
					_ = cache.Store(cacheKey, raw)
				}
			}
		}

	case "traceroute":
		if file == "" {
			return nil, fmt.Errorf("--file is required when --source=traceroute")
		}
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, fmt.Errorf("read file: %w", readErr)
		}
		measurements, err = measure.ParseRIPEAtlasTraceroute(data)
		if err != nil || len(measurements) == 0 {
			measurements, err = measure.ParseScamperJSON(data)
		}
		if err != nil {
			return nil, fmt.Errorf("parse traceroute file: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown source %q (expected \"ripe\" or \"traceroute\")", source)
	}

	if len(measurements) == 0 {
		return nil, fmt.Errorf("no measurements loaded")
	}
	return measurements, nil
}

// printScanTable prints a human-readable per-link summary table.
func printScanTable(p *tomo.Problem, sol *tomo.Solution, top int, quiet bool) {
	q := p.Quality

	// Summary line
	congested := 0
	for i := range p.Links {
		if q.IsIdentifiable(i) && sol.X.AtVec(i) > style.DelayCongestionMS {
			congested++
		}
	}
	fmt.Printf("\n%s  %s  %s  %s\n\n",
		style.Bold(fmt.Sprintf("%d links", len(p.Links))),
		style.Yellow(fmt.Sprintf("%d congested", congested)),
		"RMSE —",
		fmt.Sprintf("%.0f%% identifiable", q.IdentifiableFrac*100))

	// Build sorted index (descending by estimated delay)
	idx := make([]int, len(p.Links))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool {
		return sol.X.AtVec(idx[a]) > sol.X.AtVec(idx[b])
	})
	if top > 0 && top < len(idx) {
		idx = idx[:top]
	}

	if !quiet {
		fmt.Printf("%s %s %s %s %s\n",
			style.Bold(fmt.Sprintf("%-6s", "Link")),
			style.Bold(fmt.Sprintf("%-20s", "Endpoints")),
			style.Bold(fmt.Sprintf("%-10s", "Est(ms)")),
			style.Bold(fmt.Sprintf("%-10s", "Coverage")),
			style.Bold(fmt.Sprintf("%-8s", "Ident")))
		fmt.Println("--------------------------------------------------------------")
	}

	var identCount int
	var sumEst float64
	for _, i := range idx {
		link := p.Links[i]
		est := sol.X.AtVec(i)
		coverage := q.CoveragePerLink[i]
		identifiable := q.IsIdentifiable(i)
		if identifiable {
			identCount++
			sumEst += est
		}
		label := fmt.Sprintf("%d->%d", link.Src, link.Dst)
		fmt.Printf("%-6d %-20s %s %-10d %s\n", i, label, style.PadRight(style.ColorDelay(est), 10), coverage, style.PadRight(style.ColorIdent(identifiable), 8))
	}

	if identCount > 0 {
		mean := sumEst / float64(identCount)
		var sumSqDiff float64
		for _, i := range idx {
			if q.IsIdentifiable(i) {
				diff := sol.X.AtVec(i) - mean
				sumSqDiff += diff * diff
			}
		}
		stddev := math.Sqrt(sumSqDiff / float64(identCount))
		fmt.Printf("\nIdentifiable links: %d / %d\n", identCount, len(idx))
		fmt.Printf("Mean estimate:      %.4f ms\n", mean)
		fmt.Printf("Std dev:            %.4f ms\n", stddev)
	}
}
