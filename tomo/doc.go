// Package tomo provides network tomography solvers for inferring internal
// network state from edge measurements.
//
// The core abstraction is the Solver interface: given a Problem (routing
// matrix A, measurement vector b), a Solver produces a Solution (per-link
// estimates x, confidence intervals, identifiability mask).
//
// Available solvers: Tikhonov, NNLS, TSVD, ADMM, IRL1, VardiEM,
// Tomogravity, and Laplacian. Bootstrap and Conformal Prediction provide
// uncertainty quantification.
//
//	import "github.com/Darkroom4364/netlens/tomo"
//
//	p, _ := tomo.BuildProblemFromTopology(topo, groundTruth)
//	sol, _ := (&tomo.NNLSSolver{}).Solve(ctx, p)
package tomo
