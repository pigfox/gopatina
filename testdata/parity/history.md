# Parity history

One row per pull request that changes what `gopatina-parity` measures, so
the trend shows (docs/QUALITY.md). Rates are of the programs tested;
skipped programs are never counted as passing. Crashed is gopatina
failing with no diagnostic, a defect of its own.

| Date | Change | Tested | Skipped | Accepted | Compiled | Matched | Crashed |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-11-25 | #176, the baseline | 139 | 51 | 33 (23.7%) | 32 (23.0%) | 30 (21.6%) | 2 |
| 2026-11-25 | #177, bare returns of borrows | 139 | 51 | 33 (23.7%) | 32 (23.0%) | 30 (21.6%) | 0 |
| 2026-11-25 | untyped constants beyond their default type (gotour/concurrency/channels added to skip.txt: refused before, its output varies) | 138 | 52 | 33 (23.9%) | 33 (23.9%) | 31 (22.5%) | 0 |
