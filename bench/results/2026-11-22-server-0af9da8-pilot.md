# Test-server pilot, wall time — 2026-11-22

The first run on the test server: the 100-run pilot that sizes the speed
bar's equivalence run (docs/QUALITY.md, "The two bars"). **Inconclusive, from
noise**: the wall-time spread on this machine is far wider than the ±0.5%
margin, so this pilot neither shows nor rules out equivalence.

| Field | Value |
| --- | --- |
| Date | 2026-11-22, 03:52 to 03:54 UTC |
| GoPatina | `0af9da87949b1e710ec7a00c4b3a24a79e9d7b49` (main) |
| Machine | Intel Xeon Gold 6152 at 2.10 GHz, 7 vCPUs, 7 GB RAM: a rented virtual server (VPS) |
| OS | Linux 6.8.0-147-generic x86_64 (Ubuntu 24.04) |
| Go | go version go1.27.1 linux/amd64 |
| Rust | rustc 1.100.0 (0ef1bc386 2026-11-09), the latest stable; GoPatina's emitted crates pin rustc 1.93.0 (254b59607 2026-01-19) |
| Method | `bench/run.sh --equiv --runs 100`: 100 runs of each binary in ABBA order after 2 warm-ups each, wall time (CPU counters are not available on this machine), TOST at ±0.5% and 5σ |
| Load | load average 0.23, 0.98, 2.25 before; 1.01, 1.03, 2.12 after |
| Raw results | [2026-11-22-server-0af9da8-pilot.json](2026-11-22-server-0af9da8-pilot.json) (the run's temporary binary paths removed) |

GoPatina / hand-written Rust, with the 5σ interval:

| Case | Ratio | 5σ interval | sd of log ratios | Verdict |
| --- | --- | --- | --- | --- |
| `fib` | 0.984 | [0.948, 1.021] | 0.069 | inconclusive |
| `sieve` | 0.975 | [0.874, 1.087] | 0.204 | inconclusive |
| `suffix-sum` | 1.007 | [0.961, 1.056] | 0.087 | inconclusive |

What it shows:

- **The noise.** The standard deviation of the paired log ratios is 7% to
  20%. Even if the true difference were zero, deciding ±0.5% at 5σ would take
  about N ≈ (5 · s / δ)² pairs: 4,824 for `fib`, 41,884 for `sieve` and
  7,691 for `suffix-sum` (6,947, 60,313 and 11,075 at 6σ). For `sieve` that
  is hours of runs on this machine, and the mean ratios measured here are
  themselves well inside the noise.
- **No speed claim.** The intervals contain 1 and reach far beyond ±0.5%, so
  nothing here says GoPatina is faster, slower or equivalent.
- **What decides it instead.** CPU performance counters (user instructions
  vary by about 0.0002% run to run, so 10 to 100 runs decide), or a quieter,
  dedicated machine. Neither is available on this server yet; the project
  lead is choosing between them.
