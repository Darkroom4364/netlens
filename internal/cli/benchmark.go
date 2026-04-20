package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Darkroom4364/netlens/internal/bench"
	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
	"github.com/spf13/cobra"
)

func newBenchmarkCmd() *cobra.Command {
	var (
		topoDir          string
		noiseScale       float64
		noiseModel       string
		congestionLinks  int
		congestionFactor float64
		seed             int64
		synthetic        bool
	)

	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Run all solvers on all topologies and compare accuracy",
		Long: `Loads all GraphML files from a directory, simulates measurements with
configurable noise, runs every solver, and produces a comparison table.`,
		Example: `  netlens benchmark -t testdata/topologies
  netlens benchmark --synthetic --noise 0.2
  netlens benchmark -t ./topos --congestion-links 5 --seed 0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := measure.SimConfig{
				NoiseScale:       noiseScale,
				NoiseModel:       noiseModel,
				CongestionLinks:  congestionLinks,
				CongestionFactor: congestionFactor,
				SamplesPerPath:   3,
				Seed:             seed,
				PathFraction:     1.0,
			}

			solvers := []tomo.Solver{
				&tomo.TSVDSolver{},
				&tomo.TikhonovSolver{},
				&tomo.NNLSSolver{},
				&tomo.ADMMSolver{},
				&tomo.VardiEMSolver{},
				&tomo.TomogravitySolver{},
				&tomo.IRL1Solver{},
				&tomo.LaplacianSolver{},
			}

			var allResults []bench.BenchResult

			// Synthetic topologies.
			if synthetic {
				syntheticTopos := map[string]*topology.Graph{
					"ba-30":    topology.BarabasiAlbert(30, 3, seed),
					"ba-50":    topology.BarabasiAlbert(50, 3, seed),
					"waxman-30": topology.Waxman(30, 0.5, 0.5, seed),
					"waxman-50": topology.Waxman(50, 0.5, 0.5, seed),
				}
				for name, g := range syntheticTopos {
					results, err := bench.RunBenchmark(cmd.Context(), name, g, solvers, cfg)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: %s: %v\n", name, err)
						continue
					}
					allResults = append(allResults, results...)
				}
			}

			// GraphML file topologies.
			if !synthetic {
				files, err := filepath.Glob(filepath.Join(topoDir, "*.graphml"))
				if err != nil {
					return fmt.Errorf("glob topologies: %w", err)
				}
				if len(files) == 0 {
					return fmt.Errorf("no .graphml files found in %s", topoDir)
				}
				for _, f := range files {
					g, err := topology.LoadGraphML(f)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: skip %s: %v\n", f, err)
						continue
					}

					name := strings.TrimSuffix(filepath.Base(f), ".graphml")
					results, err := bench.RunBenchmark(cmd.Context(), name, g, solvers, cfg)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: %s: %v\n", name, err)
						continue
					}
					allResults = append(allResults, results...)
				}
			}

			fmt.Print(bench.FormatResults(allResults))
			return nil
		},
	}

	cmd.Flags().StringVarP(&topoDir, "topologies", "t", "testdata/topologies", "Directory containing .graphml files")
	cmd.Flags().Float64Var(&noiseScale, "noise", 0.1, "Noise scale (relative)")
	cmd.Flags().StringVar(&noiseModel, "noise-model", "lognormal", "Noise model: lognormal, gaussian")
	cmd.Flags().IntVar(&congestionLinks, "congestion-links", 2, "Number of congested links")
	cmd.Flags().Float64Var(&congestionFactor, "congestion-factor", 5.0, "Congestion delay multiplier")
	cmd.Flags().Int64Var(&seed, "seed", 42, "Random seed")
	cmd.Flags().BoolVar(&synthetic, "synthetic", false, "Use synthetic topologies (Barabasi-Albert, Waxman) instead of GraphML files")

	return cmd
}
