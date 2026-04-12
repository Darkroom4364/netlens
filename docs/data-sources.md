# Data Sources Reference

netlens supports five measurement sources and a local cache layer. Each produces
`tomo.PathMeasurement` values that feed the inference pipeline.

---

## 1. RIPE Atlas

Live traceroute results from the RIPE Atlas probe network via the v2 REST API.
Provides per-hop IP, RTT (ms), MPLS labels, loss, and Paris ID.

**Auth:** Set `RIPE_ATLAS_API_KEY` env var. Sent as `Authorization: Key <key>`.
Without a key, only public measurements are accessible.

**CLI flags (on `scan`):**

| Flag | Description |
|------|-------------|
| `--source=ripe` | Select RIPE Atlas source |
| `--msm <ID>` | Measurement ID (required) |
| `--start <unix>` | Result window start (default: 1 hour ago) |
| `--stop <unix>` | Result window end (default: now) |
| `--cache` | Cache results locally (see section 6) |

**Example:**

```bash
export RIPE_ATLAS_API_KEY=your-key-here
netlens scan --source=ripe --msm 1234567 --cache -m nnls -f json
```

**Gotchas:**
- Rate limiting: retries up to 3x with exponential backoff (respects `Retry-After`).
- `WaitForResults` polls with backoff capped at 30s; one-offs may take minutes.
- Only `traceroute` type measurements are parsed; ping-only are skipped.

---

## 2. Traceroute Files

Offline traceroute data from scamper, mtr, or RIPE Atlas JSON exports. Same
hop-level path data as live sources, auto-detected from the JSON structure.

**Formats:**

| Format | Tool | RTT unit | Key fields |
|--------|------|----------|------------|
| RIPE Atlas JSON | Atlas export | ms | `result[].result[].rtt`, `from` |
| Scamper JSON | `warts2json` | us | `hops[].rtt`, `hops[].addr` |
| MTR JSON | `mtr --json` | ms | `report.hubs[].Avg`, `Host` |

**CLI flags (on `scan`):**

| Flag | Description |
|------|-------------|
| `--source=traceroute` | Select file source |
| `--file <path>` | Path to JSON file (required) |
| `--max-anonymous 0.3` | Discard paths with >30% anonymous hops |

**Example:**

```bash
# Scamper warts converted to JSON
scamper -O json -c "trace -P icmp-paris" -i 8.8.8.8 > trace.json
netlens scan --source=traceroute --file trace.json -m tikhonov

# MTR JSON output
mtr --json 8.8.8.8 > mtr.json
netlens scan --source=traceroute --file mtr.json
```

**Gotchas:**
- Auto-detection tries RIPE Atlas first, then scamper. MTR is detected by its
  `{"report": ...}` envelope.
- Scamper RTTs are in **microseconds**; Atlas and MTR use **milliseconds**.
  Parsers convert automatically.
- Anonymous hops (`*`, `0.0.0.0`) are kept; `--max-anonymous` controls the
  discard threshold. MPLS labels are extracted from ICMP extensions.

---

## 3. PerfSONAR (esmond API)

Latency timeseries from PerfSONAR measurement archives via the esmond REST API.
Provides end-to-end latency points (no per-hop detail). Queries the esmond
metadata endpoint filtered by source, destination, and `event-type=packet-trace`,
then fetches timeseries from the returned `base-uri`.

**Programmatic usage** (no dedicated CLI subcommand yet):

```go
src := measure.NewPerfSONARSource("https://ps.example.com/esmond/perfsonar/archive", nil)
measurements, err := src.FetchLatency(ctx, "10.0.0.1", "10.0.0.2", 3600)
```

**Gotchas:**
- End-to-end only (no hops); topology must come from another source.
- HTTP client defaults to 30s timeout. Rate limiting: 3 retries with `Retry-After`.
- `base-uri` from metadata is a path, resolved against the archive host automatically.

---

## 4. ICMP Probing (Built-in Traceroute)

Active traceroute using ICMP echo with incrementing TTL via `pro-bing`. Full
hop-by-hop path discovery with per-hop RTT (average of multiple probes) and loss.
Defaults: max 32 hops, 2s per-hop timeout, 3 packets per hop.

**Programmatic usage** (no dedicated CLI subcommand yet):

```go
prober := measure.NewICMPProber()
m, err := prober.Probe(ctx, "8.8.8.8")

// Multiple targets concurrently
results, err := prober.ProbeMultiple(ctx, []string{"8.8.8.8", "1.1.1.1"})
```

**Gotchas:**
- Requires **root/privileged** access. On macOS: `sudo`. On Linux: `CAP_NET_RAW`.
- Non-responding hops are recorded as anonymous (not skipped).
- `ProbeMultiple` uses `errgroup`; one failure cancels all remaining probes.
- Source is always `"local"`.

---

## 5. Simulation

Synthetic measurements with known ground truth for solver validation. Produces
a `SimResult` with routing matrix, noisy/noise-free measurements, and true
per-link delays for RMSE/MAE accuracy reporting.

**CLI flags (on `simulate`):**

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --topology` | (required) | Topology Zoo GraphML file |
| `-m, --method` | `tikhonov` | Solver: tsvd, tikhonov, nnls, admm, vardi, tomogravity |
| `--noise` | `0.1` | Relative noise scale (0.1 = 10%) |
| `--noise-model` | `lognormal` | `lognormal` or `gaussian` |
| `--congestion-links` | `2` | Number of links with elevated delay |
| `--congestion-factor` | `5.0` | Delay multiplier on congested links |
| `--samples` | `3` | RTT samples per path (takes minimum) |
| `--seed` | `42` | RNG seed (0 = non-deterministic) |
| `--path-fraction` | `1.0` | Fraction of all-pairs paths to use |

**Example:**

```bash
netlens simulate -t testdata/Abilene.graphml -m nnls --noise 0.15 --noise-model gaussian
netlens simulate -t testdata/Geant2012.graphml --congestion-links 5 --seed 0
```

**Noise models:**
- **Log-normal** (default, recommended): `measurement = truth * exp(N(0, sigma))`.
  Always positive. Models heavy-tailed queueing delays realistically.
- **Gaussian**: `measurement = truth + N(0, sigma * truth)`. Can produce
  negative values (clamped to 0). Simpler but less realistic.

**Ground truth:** Link delays derive from geographic distance (~5 us/km). Nodes
without coordinates get random delays in 1-10 ms. Congestion is injected on
randomly selected links.

**Gotchas:**
- Multiple samples per path use the **minimum** (not mean), matching real RTT practice.
- `--path-fraction < 1.0` subsamples paths, reducing matrix rank.

---

## 6. Measurement Cache

File-based cache for RIPE Atlas API responses. Stores JSON at
`~/.cache/netlens/<sha256-hash>.json`, keyed by `SHA-256(msmID:start:stop)`.

**CLI flag:** `--cache` on `scan`. On hit, loads from disk. On miss, fetches
from API then stores. Entries never expire automatically.

**Example:**

```bash
# First run fetches from API and caches
netlens scan --source=ripe --msm 1234567 --cache

# Second run loads instantly from ~/.cache/netlens/
netlens scan --source=ripe --msm 1234567 --cache
```

**Gotchas:**
- Changing the time window creates a new entry, not an update.
- No TTL or eviction. Clean up manually: `rm -rf ~/.cache/netlens/`.
- Only RIPE Atlas is cached; traceroute files and simulation are already local.
- Directory created on first write with mode `0755`.
