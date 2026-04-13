package tomo

const (
	// SVDEpsilon is the near-zero threshold for filtering small singular values
	// in solver computations. Values below this are treated as numerical zero.
	SVDEpsilon = 1e-15

	// ConvergenceTolerance is the default stopping criterion for iterative
	// solvers (ADMM, IRL1, Vardi EM).
	ConvergenceTolerance = 1e-6

	// GCVMinLambda is the minimum lambda value in the GCV/L-curve search range.
	GCVMinLambda = 1e-12

	// FiberPropagationSpeed is the approximate one-way delay per km of fiber
	// optic cable, in milliseconds (~5 μs/km).
	FiberPropagationSpeed = 0.005
)
