//go:build tui

package cli

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Darkroom4364/netlens/internal/measure"
	"github.com/Darkroom4364/netlens/tomo"
	"github.com/Darkroom4364/netlens/topology"
	"github.com/Darkroom4364/netlens/internal/tui"
	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	var (
		topoFile string
		method   string
	)

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive TUI for exploring tomography results",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If topology flag provided, skip wizard and go straight to dashboard.
			if topoFile != "" {
				g, err := topology.LoadGraphML(topoFile)
				if err != nil {
					return fmt.Errorf("load topology: %w", err)
				}

				sim, err := measure.Simulate(g, measure.DefaultSimConfig())
				if err != nil {
					return fmt.Errorf("simulate: %w", err)
				}

				solver, err := getSolver(method)
				if err != nil {
					return err
				}
				sol, err := solver.Solve(cmd.Context(), sim.Problem)
				if err != nil {
					return fmt.Errorf("solve: %w", err)
				}

				allSolvers := []tomo.Solver{
					&tomo.TikhonovSolver{},
					&tomo.TSVDSolver{},
					&tomo.NNLSSolver{},
					&tomo.ADMMSolver{},
					&tomo.IRL1Solver{},
					&tomo.VardiEMSolver{},
					&tomo.TomogravitySolver{},
					&tomo.LaplacianSolver{},
				}
				idx := 0
				for i, s := range allSolvers {
					if s.Name() == solver.Name() {
						idx = i
						break
					}
				}

				model := tui.New(sim.Problem, sol, allSolvers, idx)
				p := tea.NewProgram(model, tea.WithAltScreen())
				_, err = p.Run()
				return err
			}

			// No flags — launch interactive wizard.
			model := tui.NewWizard()
			p := tea.NewProgram(model, tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}

	cmd.Flags().StringVarP(&topoFile, "topology", "t", "", "Path to Topology Zoo GraphML file (optional, skips wizard)")
	cmd.Flags().StringVarP(&method, "method", "m", "tikhonov", "Solver method: tsvd, tikhonov, nnls, admm, irl1, vardi, tomogravity, laplacian")

	return cmd
}

// Ensure tomo is used (getSolver references it via simulate.go in same package).
var _ tomo.Solver = (*tomo.TikhonovSolver)(nil)
