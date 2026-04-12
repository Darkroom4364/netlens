# Measurement Design (`netlens plan`)

## Problem

Network tomography solves `y = Ax` where `A` is a routing matrix (paths x links), `y` is
end-to-end measurements, and `x` is the unknown per-link state. The system is only solvable
when `rank(A)` equals the number of links. In practice, you cannot measure every possible
source-destination pair — probe budgets are limited, and many paths are redundant (they
traverse the same links, adding rows to `A` without increasing rank).

`netlens plan` answers: **given a fixed budget of probes, which paths should you measure to
maximize the number of identifiable links?**

## Algorithm: Greedy Rank Maximization via SVD

The design algorithm is a greedy set-cover variant that maximizes `rank(A)` one probe at a time:

1. Enumerate all shortest-path routing vectors from the topology.
2. For each iteration (up to `budget`):
   - For every candidate path not yet selected, tentatively append its routing row to `A`.
   - Compute the SVD of the augmented matrix and determine its numerical rank.
   - Pick the candidate that produces the largest rank gain.
   - If no candidate increases rank, stop early.
3. Return the ordered list of `(src, dst, rank_gain)` triples.

Rank is computed via SVD with a relative tolerance of `1e-10 * max(m, n) * sigma_max`.
This avoids counting near-zero singular values caused by floating-point noise.

If an existing `Problem` is provided (measurements already taken), the algorithm initializes
`A` with those rows and only recommends paths that add new information.

## Identifiability Analysis

`tomo.AnalyzeQuality(A)` performs a full identifiability audit of the routing matrix:

1. **SVD factorization** — decomposes `A = U * Sigma * V^T`.
2. **Rank** — count of singular values above the tolerance threshold.
3. **Condition number** — `sigma_max / sigma_min` over nonzero singular values. High values
   (>100) mean small measurement noise gets amplified into large estimation errors.
4. **Null-space detection** — for each link (column of `A`), check its projection onto the
   right singular vectors corresponding to nonzero singular values. If a link's row in `V`
   is all zeros across those columns, it lives in the null space and is **unidentifiable**.
5. **Coverage** — count of paths traversing each link. Zero-coverage links are trivially
   unidentifiable; low coverage links are poorly conditioned.

A link is unidentifiable when no combination of measured paths can isolate its metric from
its neighbors. This is a structural property of the topology and measurement plan, not a
noise issue.

## MatrixQuality Struct

```go
type MatrixQuality struct {
    Rank                int       // rank(A) — number of independently measurable link combinations
    NumLinks            int       // n — total links (columns of A)
    NumPaths            int       // m — total measurement paths (rows of A)
    ConditionNumber     float64   // cond(A) — noise amplification factor, lower is better
    IdentifiableFrac    float64   // rank/n — fraction of links that can be individually resolved
    UnidentifiableLinks []int     // link indices in the null space of A
    CoveragePerLink     []int     // per-link count of traversing paths
    SingularValues      []float64 // full singular value spectrum for diagnostics
}
```

| Field                 | Interpretation                                                     |
|-----------------------|--------------------------------------------------------------------|
| `Rank`                | Must equal `NumLinks` for full identifiability                     |
| `ConditionNumber`     | <10 excellent, 10-100 acceptable, >100 estimates become unreliable |
| `IdentifiableFrac`    | 1.0 = all links resolvable, <1.0 = null-space links exist         |
| `UnidentifiableLinks` | These links cannot be estimated — do not trust their values        |
| `CoveragePerLink`     | Links with coverage 0-1 are candidates for additional probes       |

## CLI Usage

```bash
# Recommend up to 20 probes for a Topology Zoo network
netlens plan -t testdata/Abilene.graphml

# Tighter budget
netlens plan -t testdata/Abilene.graphml -b 5
```

Example output:

```
Topology:  Abilene.graphml (11 nodes, 14 links)
Budget:    20 probes

Step   Src    Dst    RankGain    CumRank
-------------------------------------------
1      0      4      1           1
2      1      7      1           2
3      2      10     1           3
4      3      8      1           4
...
14     6      9      1           14

Total probes: 14, final rank: 14 / 14 links
Full rank achieved: all links identifiable.
```

**Reading the output:**

- Each row is one recommended measurement, in priority order.
- `RankGain` is always 0 or 1 per step (binary routing matrix), but could be >1 with
  weighted or multipath routing.
- `CumRank` shows progress toward full identifiability.
- If the algorithm stops before exhausting the budget, remaining paths are redundant.
- A rank deficit at the end means some links are structurally unidentifiable given the
  topology — no amount of additional edge probes will help.

## Integration with RIPE Atlas

Use plan output to create targeted RIPE Atlas traceroute measurements:

1. Run `netlens plan` to get the recommended `(src, dst)` pairs.
2. Map node IDs to real IP prefixes or Atlas probe IDs (topology labels correspond to
   AS names or PoP locations in Topology Zoo files).
3. Create Atlas measurements for each pair:
   ```bash
   # Example: create a traceroute from probe near node 0 to target near node 4
   netlens atlas create --type traceroute \
       --target 198.51.100.1 \
       --probe-asn 11537 \
       --one-off
   ```
4. Collect results and feed them back into `netlens scan` for inference.

The plan ensures you spend Atlas credits only on measurements that actually improve
identifiability, avoiding redundant paths that add rows to `A` without increasing rank.
