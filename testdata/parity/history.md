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
| 2026-11-26 | the verb %g of a float (gotour/basics/imports now matches; if-and-else also needs math.Pow) | 138 | 52 | 40 (29.0%) | 40 (29.0%) | 40 (29.0%) | 0 |
| 2026-11-26 | float multiply-adds fused exactly where gc fuses them on arm64 and GOAMD64=v3 (no corpus change on amd64 v1, where gc never fuses) | 138 | 52 | 40 (29.0%) | 40 (29.0%) | 40 (29.0%) | 0 |
| 2026-11-26 | a standard-library refusal clusters by the program's own call (calls filepath.Join, ...), not the internal package below it; counts unchanged | 138 | 52 | 40 (29.0%) | 40 (29.0%) | 40 (29.0%) | 0 |
| 2026-11-26 | defer of a print (gotour defer and defer-multi, gobyexample/exit now match); gobyexample/closing-channels skipped, its goroutines' prints interleave differently between Go runs | 137 | 53 | 43 (31.4%) | 43 (31.4%) | 43 (31.4%) | 0 |
| 2026-11-26 | a fused multiply-add through a local, and float32, on arm64 and GOAMD64=v3 (no corpus change at amd64 v1) | 137 | 53 | 43 (31.4%) | 43 (31.4%) | 43 (31.4%) | 0 |
| 2026-11-27 | NaN payloads match gc where math.Float64bits reads them (no corpus program reads NaN bits) | 137 | 53 | 43 (31.4%) | 43 (31.4%) | 43 (31.4%) | 0 |
| 2026-11-27 | math.Exp, bit for bit on every chip (no corpus program calls it) | 137 | 53 | 43 (31.4%) | 43 (31.4%) | 43 (31.4%) | 0 |
| 2026-11-27 | math.Log, bit for bit on every chip (no corpus program calls it) | 137 | 53 | 43 (31.4%) | 43 (31.4%) | 43 (31.4%) | 0 |
| 2026-11-27 | math.Sin and math.Cos (#202) and math.Pow (#206), measured here: 3 more programs match, among them gotour/flowcontrol/if-and-else and if-with-a-short-statement (math.Pow) | 137 | 53 | 46 (33.6%) | 46 (33.6%) | 46 (33.6%) | 0 |
| 2026-11-27 | printing structs with %v (Println, Print, Sprint): gotour/moretypes/structs and struct-pointers now match | 137 | 53 | 48 (35.0%) | 48 (35.0%) | 48 (35.0%) | 0 |
| 2026-11-27 | the verb %q of a string, as strconv.Quote: gotour/basics/zero and methods/type-switches now match | 137 | 53 | 50 (36.5%) | 50 (36.5%) | 50 (36.5%) | 0 |
| 2026-11-27 | panic(err) and Stringers: no program matches yet. In 8 of the 9 that panic with an error, the error comes from os or strconv (*fs.PathError, *strconv.NumError), whose Error method is not yet known not to panic, and they also call file APIs; gobyexample/enums needs maps | 137 | 53 | 50 (36.5%) | 50 (36.5%) | 50 (36.5%) | 0 |
| 2026-11-28 | closures, step 1 of RFC 0018 (called where they are made): no program matches yet; the corpus's closures escape, recurse or take strings, steps 2 to 4, and their refusals now say which | 137 | 53 | 50 (36.5%) | 50 (36.5%) | 50 (36.5%) | 0 |
| 2026-11-28 | function-typed parameters, step 2 of RFC 0018: no program matches yet; what is left of the corpus's closures is function-typed results and variables (step 3) and closures over strings | 137 | 53 | 50 (36.5%) | 50 (36.5%) | 50 (36.5%) | 0 |
| 2026-11-28 | returned closures and function values, step 3 of RFC 0018: gobyexample/closures, gotour/moretypes/function-closures and gotour/solutions/fib now match | 137 | 53 | 53 (38.7%) | 53 (38.7%) | 53 (38.7%) | 0 |
| 2026-11-28 | recursive closures as nested fns, step 4 of RFC 0018: gobyexample/recursion now matches | 137 | 53 | 54 (39.4%) | 54 (39.4%) | 54 (39.4%) | 0 |
| 2026-11-28 | deferred closures, step 5 of RFC 0018: no program matches yet; the corpus programs that defer a closure also need sync (mutexes, mutex-counter) | 137 | 53 | 54 (39.4%) | 54 (39.4%) | 54 (39.4%) | 0 |
| 2026-11-28 | local maps, step 1 of RFC 0019: gotour/moretypes/mutating-maps now matches | 137 | 53 | 55 (40.1%) | 55 (40.1%) | 55 (40.1%) | 0 |
| 2026-11-28 | package-level maps and struct values, step 2 of RFC 0019: gotour/moretypes/map-literals and map-literals-continued now match | 137 | 53 | 57 (41.6%) | 57 (41.6%) | 57 (41.6%) | 0 |
| 2026-11-28 | range over a map, step 3 of RFC 0019: no program matches yet; the corpus programs that range over a map print in its order (skipped) or need more (gobyexample/maps: clear and maps.Equal) | 137 | 53 | 57 (41.6%) | 57 (41.6%) | 57 (41.6%) | 0 |
| 2026-11-28 | math functions as function values: gotour/moretypes/function-values now matches | 137 | 53 | 58 (42.3%) | 58 (42.3%) | 58 (42.3%) | 0 |
