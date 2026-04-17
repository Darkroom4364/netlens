package tomo

import (
	"fmt"
	"time"

	"gonum.org/v1/gonum/mat"
)

// BuildProblem constructs a Problem from a topology and measurement data.
// It builds the routing matrix A and measurement vector b, then analyzes quality.
func BuildProblem(topo Topology, paths []PathSpec, measurements []float64) (*Problem, error) {
	nLinks := topo.NumLinks()
	nPaths := len(paths)

	if nPaths != len(measurements) {
		return nil, fmt.Errorf("path count (%d) != measurement count (%d)", nPaths, len(measurements))
	}
	if nPaths == 0 {
		return nil, fmt.Errorf("no paths provided")
	}
	if nLinks == 0 {
		return nil, fmt.Errorf("no links in topology")
	}

	// Build routing matrix A (m × n)
	aData := make([]float64, nPaths*nLinks)
	for i, p := range paths {
		for _, linkID := range p.LinkIDs {
			if linkID < 0 || linkID >= nLinks {
				return nil, fmt.Errorf("path %d references invalid link %d", i, linkID)
			}
			aData[i*nLinks+linkID] = 1.0
		}
	}

	A := mat.NewDense(nPaths, nLinks, aData)
	B := mat.NewVecDense(nPaths, measurements)

	quality := AnalyzeQuality(A)

	return &Problem{
		Topo:    topo,
		A:       A,
		B:       B,
		Paths:   paths,
		Links:   topo.Links(),
		Quality: quality,
	}, nil
}

// BuildProblemFromTopology constructs a Problem using all-pairs shortest paths
// and synthetic delay measurements from the given ground truth.
func BuildProblemFromTopology(topo Topology, groundTruth []float64) (*Problem, error) {
	paths := topo.AllPairsShortestPaths()
	if len(paths) == 0 {
		return nil, fmt.Errorf("no reachable paths in topology")
	}

	nLinks := topo.NumLinks()
	if len(groundTruth) != nLinks {
		return nil, fmt.Errorf("ground truth length (%d) != link count (%d)", len(groundTruth), nLinks)
	}

	// Compute synthetic end-to-end measurements: b = A * x_true
	measurements := make([]float64, len(paths))
	for i, p := range paths {
		for _, linkID := range p.LinkIDs {
			measurements[i] += groundTruth[linkID]
		}
	}

	return BuildProblem(topo, paths, measurements)
}

// BuildProblemFromMeasurements constructs a Problem from PathMeasurements.
// Uses minimum RTT from each measurement as the end-to-end delay.
func BuildProblemFromMeasurements(topo Topology, pathMeasurements []PathMeasurement, pathSpecs []PathSpec) (*Problem, error) {
	if len(pathMeasurements) != len(pathSpecs) {
		return nil, fmt.Errorf("measurement count (%d) != path spec count (%d)",
			len(pathMeasurements), len(pathSpecs))
	}

	measurements := make([]float64, len(pathMeasurements))
	for i, m := range pathMeasurements {
		minRTT := m.MinRTT()
		measurements[i] = float64(minRTT) / float64(time.Millisecond) // convert to ms
	}

	return BuildProblem(topo, pathSpecs, measurements)
}
