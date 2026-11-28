# Allocators for static musl binaries, 2026-11-27 (not official)

The question was whether patina binaries can be static musl by default, as
RFC 0001 plans, without getting slower. musl's allocator is known to be
slower than glibc's, so this run compares it with glibc and with two
allocators a static build could use instead. The answer, for now, is no:
`-static` is opt-in, and gnu stays the default.

## Method

- **Programs.** GoPatina's crates (`gopatina build -emit-rust`, at main
  6af2387) for the three bench cases that allocate in their hot loop:
  `binary-trees`, `cycles` and `iface-keep`. The hand-written Rust and Go
  versions are not in this comparison; only the allocator changes.
- **Builds.** Each crate was built four ways with rustc 1.93.0 (the pin),
  in release mode:
  - **gnu**: `x86_64-unknown-linux-gnu`, glibc's malloc (today's default);
  - **musl**: `x86_64-unknown-linux-musl`, static, musl's mallocng;
  - **mimalloc**: `#[global_allocator]` mimalloc 0.1.52. It is C, and
    building it for musl needs a musl C compiler (`x86_64-linux-musl-gcc`),
    which this machine does not have. These binaries are therefore **gnu
    builds with mimalloc**, a stand-in for the allocator's speed;
  - **dlmalloc**: `x86_64-unknown-linux-musl`, static, with the pure-Rust
    dlmalloc 0.2.14 (`GlobalDlmalloc`).
- **Runs.** Every output was first checked against the case's `expected`.
  Then 7 rounds were run, each running the four binaries one after
  another. Wall time and peak RSS came from GNU time.
- **Machine.** Intel Core i7-8650U, 8 threads, Linux 7.0. The laptop was
  in use, with a load average of 2.2 to 3.2, so read these numbers as
  directions. The intervals below are the min and max of the 7 runs.

## Results

| Case | Build | Wall time, median [min, max] | Peak RSS |
| --- | --- | --- | --- |
| `binary-trees` | gnu | 3.71 s [3.53, 3.82] | 34,920 KiB |
| | musl | 4.64 s [4.40, 5.47] | 35,544 KiB |
| | mimalloc (gnu) | 1.03 s [0.95, 1.36] | 22,908 KiB |
| | dlmalloc | 5.04 s [4.83, 6.05] | 33,060 KiB |
| `cycles` | gnu | 0.02 s [0.02, 0.03] | 2,548 KiB |
| | musl | 0.11 s [0.10, 0.14] | 544 KiB |
| | mimalloc (gnu) | 0.02 s [0.02, 0.03] | 12,728 KiB |
| | dlmalloc | 0.02 s [0.01, 0.02] | 744 KiB |
| `iface-keep` | gnu | 0.21 s [0.21, 0.35] | 9,284 KiB |
| | musl | 0.49 s [0.44, 0.58] | 7,592 KiB |
| | mimalloc (gnu) | 0.10 s [0.10, 0.12] | 14,772 KiB |
| | dlmalloc | 0.52 s [0.44, 0.66] | 14,744 KiB |

## Reading

- **musl's allocator is slower than glibc's** on all three cases:
  binary-trees took 1.25 times as long, iface-keep 2.3 times. `cycles` is
  too short at 0.02 s to rank, but musl took 0.11 s there.
- **dlmalloc does not help.** It is as slow as musl's allocator or slower,
  except on the very short `cycles`.
- **mimalloc is the fastest**: binary-trees took 0.28 times glibc's time
  and iface-keep 0.48 times. But it is C, so a static build with it needs
  a musl C compiler on every machine that builds. That breaks RFC 0001's
  rule that a static build involves no host C toolchain, and it gives every
  crate a dependency, where today they have none.

## Decision

The project lead's decision (A2, 2026-11-27):

- `-static` builds Rust's musl target on linux/amd64 and linux/arm64,
  linked by `rust-lld` with Rust's own musl runtime objects. It needs no C
  toolchain and is reproducible: two builds in different directories were
  byte-identical.
- The default stays gnu until a fast allocator for static builds is
  settled, so nothing gets slower and no C toolchain is required.
