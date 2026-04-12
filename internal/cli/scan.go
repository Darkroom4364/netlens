package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/Darkroom4364/netlens/internal/format"
	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/internal/tomo"
	"github.com/Darkroom4364/netlens/internal/topology"
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
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run end-to-end tomography on real measurement data",
		Long: `Fetches traceroute measurements from RIPE Atlas or a local file,
infers the network topology, builds the routing matrix, runs identifiability
analysis, solves the inverse problem, and outputs per-link estimates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Load measurements
			var measurements []tomo.PathMeasurement
			var err error

			switch source {
			case "ripe":
				if msmID == 0 {
					return fmt.Errorf("--msm is required when --source=ripe")
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
						return fmt.Errorf("load cache: %w", loadErr)
					}
					if err = json.Unmarshal(raw, &measurements); err != nil {
						return fmt.Errorf("parse cached results: %w", err)
					}
					fmt.Println("Loaded results from cache")
				} else {
					apiKey := os.Getenv("RIPE_ATLAS_API_KEY")
					src := measure.NewRIPEAtlasSource(apiKey, "", nil)
					ctx := context.Background()
					measurements, err = src.FetchResults(ctx, msmID, start, stop)
					if err != nil {
						return fmt.Errorf("fetch RIPE Atlas results: %w", err)
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
					return fmt.Errorf("--file is required when --source=traceroute")
				}
				data, readErr := os.ReadFile(file)
				if readErr != nil {
					return fmt.Errorf("read file: %w", readErr)
				}
				// Try RIPE Atlas format first, then scamper
				measurements, err = measure.ParseRIPEAtlasTraceroute(data)
				if err != nil || len(measurements) == 0 {
					measurements, err = measure.ParseScamperJSON(data)
				}
				if err != nil {
					return fmt.Errorf("parse traceroute file: %w", err)
				}

			default:
				return fmt.Errorf("unknown source %q (expected \"ripe\" or \"traceroute\")", source)
			}

			if len(measurements) == 0 {
				return fmt.Errorf("no measurements loaded")
			}
			fmt.Printf("Loaded %d measurements from %s\n", len(measurements), source)

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

			fmt.Printf("Topology:       %d nodes, %d links\n", graph.NumNodes(), graph.NumLinks())
			fmt.Printf("Paths:          %d (of %d measurements)\n", len(pathSpecs), len(measurements))

			// 3. Build routing matrix + Problem
			problem, err := tomo.BuildProblemFromMeasurements(graph, accepted, pathSpecs)
			if err != nil {
				return fmt.Errorf("build problem: %w", err)
			}

			// 4. Identifiability analysis (already computed in BuildProblem)
			q := problem.Quality
			fmt.Printf("Matrix rank:    %d / %d (identifiable: %.0f%%)\n",
				q.Rank, q.NumLinks, q.IdentifiableFrac*100)
			fmt.Printf("Condition:      %.2f\n", q.ConditionNumber)

			// 5. Solve with selected method
			solver := getSolver(method)
			sol, err := solver.Solve(problem)
			if err != nil {
				return fmt.Errorf("solve: %w", err)
			}

			fmt.Printf("Solver:         %s\n", sol.Method)
			fmt.Printf("Duration:       %v\n", sol.Duration)
			fmt.Printf("Residual:       %.6f\n\n", sol.Residual)

			// 6. Output results
			if outputFormat == "table" {
				printScanTable(problem, sol)
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
	cmd.Flags().StringVarP(&method, "method", "m", "tikhonov", "Solver method: tsvd, tikhonov, nnls")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "table", "Output format: json, csv, dot, table")

	now := time.Now().Unix()
	oneHourAgo := now - 3600
	cmd.Flags().Int64Var(&start, "start", oneHourAgo, "UNIX timestamp for RIPE Atlas result window start")
	cmd.Flags().Int64Var(&stop, "stop", now, "UNIX timestamp for RIPE Atlas result window stop")
	cmd.Flags().Float64Var(&maxAnonymous, "max-anonymous", 0.3, "Max anonymous hop fraction before discarding path")
	cmd.Flags().BoolVar(&useCache, "cache", false, "Cache RIPE Atlas results locally (~/.cache/netlens/)")

	_ = cmd.MarkFlagRequired("source")

	return cmd
}

// printScanTable prints a human-readable per-link summary table.
func printScanTable(p *tomo.Problem, sol *tomo.Solution) {
	q := p.Quality

	fmt.Printf("%-6s %-20s %-10s %-10s %-8s\n", "Link", "Endpoints", "Est(ms)", "Coverage", "Ident")
	fmt.Println("--------------------------------------------------------------")

	var identCount int
	var sumEst float64
	for i, link := range p.Links {
		est := sol.X.AtVec(i)
		coverage := q.CoveragePerLink[i]
		ident := "yes"
		if !q.IsIdentifiable(i) {
			ident = "NO"
		} else {
			identCount++
			sumEst += est
		}
		label := fmt.Sprintf("%d->%d", link.Src, link.Dst)
		fmt.Printf("%-6d %-20s %-10.3f %-10d %-8s\n", i, label, est, coverage, ident)
	}

	if identCount > 0 {
		mean := sumEst / float64(identCount)
		// Compute stddev of identifiable link estimates
		var sumSqDiff float64
		for i := range p.Links {
			if q.IsIdentifiable(i) {
				diff := sol.X.AtVec(i) - mean
				sumSqDiff += diff * diff
			}
		}
		stddev := math.Sqrt(sumSqDiff / float64(identCount))
		fmt.Printf("\nIdentifiable links: %d / %d\n", identCount, len(p.Links))
		fmt.Printf("Mean estimate:      %.4f ms\n", mean)
		fmt.Printf("Std dev:            %.4f ms\n", stddev)
	}
}
