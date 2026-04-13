# Solver Reference

netlens ships nine solvers for the network tomography inverse problem `y = Ax + e`, where `A` is the routing matrix, `x` is per-link metrics, and `y` is end-to-end measurements, plus Bootstrap CI and Conformal Prediction for uncertainty quantification. See [references.md](references.md) for the full list of academic papers underlying each solver.

## Core Types

**Problem** (`tomo.Problem`) — holds routing matrix `A`, measurement vector `B`, optional per-measurement `Weights`, and precomputed `MatrixQuality` (rank, condition number, identifiability mask, coverage).

**Solution** (`tomo.Solution`) — holds per-link estimates `X`, optional `Confidence` intervals, `Identifiable` mask, `Residual` norm, solver `Method` name, `Duration`, and solver-specific `Metadata`.

**Solver interface** — `Name() string` and `Solve(p *Problem) (*Solution, error)`. All nine solvers implement this.

---

## Solvers

### 1. Tikhonov (`tikhonov`)

L2-regularized least squares. The workhorse default.

**Formulation:** `min ||Ax - b||^2 + lambda * ||x||^2`
**Solution:** `x = V * diag(sigma_j / (sigma_j^2 + lambda)) * U^T * b` (SVD-based)

| Parameter | Default | Notes |
|-----------|---------|-------|
| `Lambda`  | 0 (auto via GCV) | Generalized Cross-Validation over 100 log-spaced candidates |

- **Non-negativity:** No. Can produce negative estimates.
- **When to use:** General-purpose first choice. Works well for smooth link metric vectors with moderate noise. Robust to ill-conditioning.
- **RMSE:** < 0.01 on noise-free systems; typically 0.5-2.0 ms with realistic noise.

### 2. NNLS (`nnls`)

Non-negative least squares via Lawson-Hanson active-set algorithm.

**Formulation:** `min ||Ax - b||_2  subject to  x >= 0`

| Parameter | Default | Notes |
|-----------|---------|-------|
| `MaxIter` | `3 * numLinks` | Active-set iterations |

- **Non-negativity:** Yes. Hard constraint.
- **When to use:** Link delay or loss estimation where negative values are physically meaningless. Uses QR-based inner solves (never forms A^T A).
- **RMSE:** < 0.01 noise-free; comparable to Tikhonov with noise but no negative artifacts.

### 3. Truncated SVD (`tsvd`)

Pseudoinverse with small singular values discarded to suppress noise amplification.

**Formulation:** `x = V_k * Sigma_k^{-1} * U_k^T * b` (keep top k components)

| Parameter | Default | Notes |
|-----------|---------|-------|
| `TruncationRank` | 0 (auto) | Discrepancy principle: keep sigma_j > sigma_max * sqrt(eps) * max(m,n) |

- **Non-negativity:** No.
- **When to use:** When you want explicit rank control or the routing matrix has a clear spectral gap. Good for diagnosis — metadata reports `truncation_rank` and all singular values.
- **RMSE:** < 0.01 noise-free; sensitive to rank selection under noise.

### 4. ADMM (`admm`)

L1-minimization via Alternating Direction Method of Multipliers. Compressed sensing approach.

**Formulation:** `min lambda * ||x||_1 + (1/2) * ||Ax - b||^2`
Uses soft-thresholding z-update and Cholesky-factored x-update.

| Parameter | Default | Notes |
|-----------|---------|-------|
| `Lambda`  | `0.1 * \|\|A^T b\|\|_inf` | L1 penalty weight |
| `Rho`     | 1.0 | ADMM augmented Lagrangian penalty |
| `MaxIter` | 200 | Converges when primal & dual residuals < 1e-6 |

- **Non-negativity:** No (but L1 promotes sparsity which often yields non-negative solutions).
- **When to use:** Sparse congestion — few links are degraded while most are fine. Produces sparse solutions where many link estimates are exactly zero.
- **RMSE:** Best when ground truth is sparse; can over-shrink in dense-delay scenarios.

### 5. Vardi EM (`vardi-em`)

Expectation-Maximization algorithm from Vardi (1996). Iteratively distributes path measurements to links proportionally to current estimates.

**Formulation:** Iterative — E-step distributes `b_i` to links weighted by `A(i,j) * x_j / sum`, M-step averages contributions.

| Parameter | Default | Notes |
|-----------|---------|-------|
| `MaxIter`   | 500  | EM iterations |
| `Tolerance` | 1e-6 | Convergence on max relative change |

- **Non-negativity:** Yes. Inherent — initialized at 1.0, multiplicative updates stay positive.
- **When to use:** Classic network tomography method. Good baseline. Simple, no matrix factorization needed. Works well when the routing matrix is binary (0/1).
- **RMSE:** < 0.01 noise-free; can be slower to converge than direct methods.

### 6. Tomogravity (`tomogravity`)

Gravity-model prior plus Tikhonov residual correction.

**Formulation:** Two-step: (1) `prior_j = mean(b_i / pathLen_i)` for paths through link j, (2) `x = prior + tikhonov_solve(b - A*prior)`, clamped to non-negative.

| Parameter | Default | Notes |
|-----------|---------|-------|
| `Lambda`  | 0 (auto via GCV) | Regularization for residual correction step |

- **Non-negativity:** Yes. Post-hoc clamping `max(x, 0)`.
- **When to use:** When you have a reasonable expectation that link metrics are proportional to traffic load. The gravity prior gives a sensible starting point, reducing dependence on regularization.
- **RMSE:** Often best overall — the prior reduces effective noise. Strong on real-world topologies.

### 7. IRL1 (`irl1`)

Iterative Reweighted L1 minimization. Sharpens sparse recovery by adaptively penalizing near-zero components more heavily than large ones.

**Formulation:** Outer loop reweights `w_j = 1 / (|x_j| + epsilon)`, inner loop solves `min sum(w_j |x_j|) + (1/2) ||Ax - b||^2` via ADMM.

| Parameter | Default | Notes |
|-----------|---------|-------|
| `MaxOuterIter` | 5 | Reweighting iterations |
| `MaxInnerIter` | 100 | ADMM iterations per reweight |
| `Rho` | 1.0 | ADMM augmented Lagrangian penalty |
| `Epsilon` | 0.1 | Reweighting stability parameter |

- **Non-negativity:** No (but stronger sparsity than plain L1 often yields non-negative solutions).
- **When to use:** When ADMM's uniform L1 penalty over-shrinks large congested links. IRL1 better approximates L0 sparsity, recovering both the support and magnitude of sparse congestion patterns.
- **RMSE:** Strictly better than ADMM when ground truth is sparse; diminishing returns on dense scenarios.

### 8. Laplacian (`laplacian`)

Graph-Laplacian-regularized least squares. Uses the network topology as a structural prior so that adjacent links have similar estimates.

**Formulation:** `min ||Ax - b||^2 + lambda * ||Lx||^2` where `L` is the link-graph Laplacian.

| Parameter | Default | Notes |
|-----------|---------|-------|
| `Lambda` | 0 (auto) | Regularization weight; auto-selected over 40 log-spaced candidates |

- **Non-negativity:** No.
- **When to use:** When you expect spatially smooth link metrics — e.g., regional congestion affecting neighboring links. Requires topology (`Problem.Topo` must be set).
- **RMSE:** Outperforms Tikhonov when spatial smoothness holds; can over-smooth isolated hotspots.

### 9. Bootstrap (meta-solver)

Not a solver itself — wraps any solver to produce confidence intervals via resampling.

**Formulation:** Resample rows of `(A, b)` with replacement `N` times, solve each, compute percentile-based CIs.

| Parameter | Default | Notes |
|-----------|---------|-------|
| `NumSamples` | 100  | Bootstrap iterations |
| `Alpha`      | 0.05 | Significance level (0.05 = 95% CI) |
| `Seed`       | 0 (random) | For reproducibility |

**Usage:**
```go
sol, err := tomo.Bootstrap(problem, &tomo.TikhonovSolver{}, tomo.BootstrapConfig{
    NumSamples: 200,
    Alpha:      0.05,
})
// sol.Confidence[j] = half-width of 95% CI for link j
```

The `Confidence` field on Solution is a `*mat.VecDense` where each element is `(hi - lo) / 2` from the percentile interval. Failed bootstrap samples are silently skipped.

### 10. Conformal Prediction (meta-solver)

Distribution-free prediction intervals via split conformal prediction. Faster than Bootstrap (single solve) with finite-sample marginal coverage guarantee.

**Formulation:** Split paths into training and calibration sets. Solve on training set, compute residuals on calibration set, use quantile of absolute residuals as interval half-width.

| Parameter | Default | Notes |
|-----------|---------|-------|
| `CalibrationFrac` | 0.2 | Fraction of paths held out for calibration |
| `Alpha` | 0.05 | Significance level (0.05 = 95% CI) |
| `Seed` | 0 (random) | For reproducibility |

**Usage:**
```go
sol, err := tomo.Conformal(problem, &tomo.TikhonovSolver{}, tomo.ConformalConfig{
    CalibrationFrac: 0.2,
    Alpha:           0.05,
})
// sol.Confidence[j] = conformal interval half-width for link j
```

- **When to use:** When you need fast UQ without resampling overhead. Single solve + calibration step. Coverage guarantee holds regardless of the true distribution.

---

## How to Choose a Solver

```
Start here
  |
  v
Is the link metric non-negative (delay, loss)?
  |-- No  --> Tikhonov or TSVD
  |-- Yes
       |
       v
     Do you expect sparse congestion (few bad links)?
       |-- Yes --> ADMM (L1 sparsity)
       |-- No
            |
            v
          Do you have good path diversity (rank(A) ~ n)?
            |-- Yes --> NNLS (exact non-negative LS)
            |-- No  --> Tomogravity (prior helps underdetermined case)
                        or Vardi EM (simple, no factorization)

For confidence intervals: wrap any of the above with Bootstrap.
For diagnostics: use TSVD to inspect singular value spectrum.
```

**Rules of thumb:**
- Always run `tomo.AnalyzeQuality()` first. If `IdentifiableFrac < 1.0`, no solver can recover unidentifiable links.
- Tomogravity is the safest default for real-world data.
- NNLS is the most principled for delay estimation.
- ADMM shines when you know congestion is localized.
- Tikhonov is fastest and simplest for exploration.
- TSVD metadata (`truncation_rank`, `singular_values`) is invaluable for debugging ill-conditioned problems.
