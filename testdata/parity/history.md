# Parity history

One row per pull request that changes what `gopatina-parity` measures, so
the trend shows (docs/QUALITY.md). Rates are of the programs tested;
skipped programs are never counted as passing. Crashed is gopatina
failing with no diagnostic, a defect of its own.

| Date | Change | Tested | Skipped | Accepted | Compiled | Matched | Crashed |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-11-25 | #176, the baseline | 139 | 51 | 33 (23.7%) | 32 (23.0%) | 30 (21.6%) | 2 |
| 2026-11-25 | #177, bare returns of borrows | 139 | 51 | 33 (23.7%) | 32 (23.0%) | 30 (21.6%) | 0 |
| 2026-11-25 | #178, untyped constants beyond their default type (gotour/concurrency/channels added to skip.txt: refused before, its output varies) | 138 | 52 | 33 (23.9%) | 33 (23.9%) | 31 (22.5%) | 0 |
| 2026-11-25 | %v of an interface checks the pointers it can hold (2 wrong outputs now refused: accepted and compiled fall, matched does not) | 138 | 52 | 31 (22.5%) | 31 (22.5%) | 31 (22.5%) | 0 |
| 2026-11-26 | a borrow lent back through a local or a named result (no corpus program changes: the refusal was met inside std code with other blockers) | 138 | 52 | 31 (22.5%) | 31 (22.5%) | 31 (22.5%) | 0 |
| 2026-11-26 | math provided, the functions exact by IEEE 754 (Sqrt, Abs, Floor, Ceil, Trunc and the bit functions): 8 programs blocked only by math now match | 138 | 52 | 39 (28.3%) | 39 (28.3%) | 39 (28.3%) | 0 |
| 2026-11-26 | private_interfaces allowed where Dyn holds main's unexported structs (clippy clean 36 to 37 of 39; nothing else changes) | 138 | 52 | 39 (28.3%) | 39 (28.3%) | 39 (28.3%) | 0 |
