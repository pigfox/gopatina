# RFC 0015: Measuring development speed

- **Status:** Accepted (#111; decisions 2026-11-23)
- **Author(s):** GoPatina contributors
- **Created:** 2026-11-22
- **Discussion:** the pull request that adds this file
- **Supersedes / superseded by:** none

## Summary

GoPatina's goal is Go's development speed with Rust's output. Nothing has
measured that yet, and GoPatina makes no claim about development time. This
RFC fixes the method of an experiment before any run:
- the same specified programs are written two ways, in Rust by hand and in
  Go with GoPatina;
- each version is written against the same acceptance tests;
- the order is randomized;
- each version records the time to all tests passing, and the other
  measures below.

Runs by AI agents and runs by human developers are reported separately.
Every run is published, including the ones Rust wins. The protocol is
fixed here, before results exist, so that it cannot be tuned to them.

## Motivation

The home page says that writing Go and getting Rust is faster than writing
Rust, and that this is the design, not a measurement (the
[home page](https://gopatina.org/#development-speed) and the
[roadmap](https://gopatina.org/roadmap#development-speed)). Sponsors and
users will ask how much faster, and for which programs. An honest answer
needs:
- a fixed protocol, chosen before the data;
- tasks that include programs where ownership is hard, not only ones
  GoPatina handles easily;
- every run published;
- a clear line between what an AI agent did and what a person would do.

## Guide-level explanation

A **task** is a small program with a written specification and acceptance
tests: inputs on standard input and the exact expected standard output and
exit status. The tests are language-neutral: they run a binary, so the same
tests judge both versions.

A **run** is one developer, a person or an agent, doing one task in one
**arm**:
- **R:** hand-written Rust, on stable Rust with cargo; clippy, rust-analyzer
  and the standard library documentation are allowed.
- **G:** Go built with `gopatina build -patina`; `gopatina build -patina
  -explain` and GoPatina's documentation are allowed.

A run ends when the binary passes every acceptance test, or at the time
cap, when it counts as **not finished**. In both arms, no `unsafe` code
and no crates beyond the standard library are allowed, since GoPatina's
output uses neither.

A small harness, `experiments/devspeed/run`, starts and stops the clock,
runs the build, the tests and the counters, and writes a JSON log of the
run.

## Reference-level explanation

### Tasks

- **Six tasks to start**, written and published, with their tests, before
  the first run. They are not changed once a run has used them; a fix
  makes a new task version, and runs are reported per version.
- They cover four kinds:
  - text processing (parsing and formatting lines);
  - a rules engine with errors (like `examples/rust-interop`);
  - numeric loops over slices;
  - **pointer-shaped data**: a tree, and a graph with back pointers.

  Pointer-shaped data is where Rust's ownership costs most, and where
  GoPatina's own rules (RFC 0013) are most likely to get in the way, so
  including it can go against GoPatina.
- Every task fits inside the patina subset, or arm G could not be done.
  That is a limit of the experiment, stated with every result: it measures
  programs GoPatina can build today, not Go in general.
- Each task has a reference solution in each arm, written before the runs,
  to show it can be done in the time cap. Reference solutions are not shown
  to the developers.

### Order and assignment

- **Agents.** Each run is a fresh session with no memory of other runs, so
  one agent model may do every task in both arms. The order of the
  task-arm pairs is randomized from a seed published with the results.
- **People.** A person who has written a task once knows its solution, so
  no person does the same task in both arms. Each person does half the
  tasks in R and half in G. Which half is assigned by a balanced design
  (a Latin square over tasks and arms), with the starting arm alternated
  between people. Each task is then done in each arm by different people,
  and comparisons are between people within a task.

### Measures

For each run:

1. **Time to green** (primary): wall time from the start of the run to the
   first build that passes every acceptance test.
   - For agents, wall time depends on the service's speed, so agent runs
     also report the number of turns and of tool calls. Those are the
     agent's primary measures; wall time is secondary.
2. **Build and fix cycles**: the number of builds that failed (to compile,
   to pass the borrow check, or to pass the tests), counted by the
   harness.
3. **Ownership errors**: compile errors about ownership and borrowing.
   - In R: rustc's borrow and lifetime errors, E0382, E0499, E0502, E0505,
     E0506, E0597, E0716 and the other E05xx and E07xx ownership codes. The
     list is fixed in the harness.
   - In G: GoPatina's borrow diagnostics.
   - Each error is counted once per build in which it appears.
4. **Size**: lines of code that are neither blank nor comment, for the
   developer's own source only (not generated Rust).
5. **The result's speed and memory**: the run time and peak memory of the
   finished binary on the task's benchmark input, measured as in
   `bench/README.md`.
6. **Not finished**: the run reached the time cap, 2 hours for people and a
   fixed turn limit for agents, set in the harness before the first run.

### Analysis

- For each task, the primary comparison is the ratio G / R of time to
  green, from the geometric means of the runs. It is reported with an
  interval at 95% (Student's t on the log times).
  - Agents: within the task, paired by run index.
  - People: between the two groups.
- Across tasks, the result is the geometric mean of the per-task ratios,
  with its interval. Every task's own ratio is also shown, so a task where
  Rust wins is visible, not averaged away.
- Unfinished runs count at the cap. This favours the slower arm, so the
  result is also shown with them left out, and their number is reported
  per arm.
- The same is done for build cycles, turns and tool calls (agents) and
  ownership errors. Size and the result's speed are reported per task,
  without a combined number.
- Nothing is called "faster" unless the whole interval is below 1, and no
  figure is published without the runs that produced it.

### Agent runs

- **Reported as agent runs**, never as human productivity. An agent's time
  to green says how well a model does each arm, not how a person would do.
- **Recorded with each run:**
  - the model's name and version;
  - the date;
  - the prompt, which is the same for both arms apart from the arm's tools
    and documentation;
  - the full transcript.
- **A known bias.** Models have seen a great deal of Rust and almost no
  GoPatina, although they know Go well. That bias is not corrected; it is
  stated with the results.
- **Who runs them.** The project that builds GoPatina runs the agent runs,
  and its own agent may be the model used. That is a conflict of interest,
  so the prompts, seeds and transcripts are published, and anyone can
  repeat a run.

### Human runs

- **Who.** Developers are invited publicly. Each records their experience
  in Go and in Rust (years, and a self-rating) before their first run, and
  results are broken down by it.
- **Consent and privacy.** Participation is voluntary. A person's runs are
  published under a pseudonym unless they choose their name, and no screen
  or keystroke recording is made. The harness records only times, build
  results and the counts above.
- **Before the first run** each person takes a short warm-up task in each
  arm, which is not counted, so that the first counted run doesn't measure
  setting up a toolchain.

### Publication

- Results live in `experiments/devspeed/results/`:
  - one JSON file per run;
  - a summary per batch of runs, with the tasks' versions, the harness
    version, GoPatina's commit, the Go and Rust versions and the seed.
- On the site they go in an `/evidence` section, with a blog post per
  milestone.
- A headline number appears only with its interval, the count of runs
  behind it, and whether the runs were by agents or people.

## Compatibility

This RFC changes no part of GoPatina. It adds `experiments/devspeed/` (the
tasks, tests, reference solutions, harness and results) and, once results
exist, a site section.

## Drawbacks

- **Small samples.** A few people or agent runs give wide intervals. The
  results may say little at first, and are published anyway.
- **The subset.** Tasks must fit the subset, so the experiment cannot
  speak for Go programs GoPatina doesn't accept yet.
- **Cost.** The work of writing and maintaining tasks, tests and reference
  solutions in two languages.

## Rationale and alternatives

- **Surveys or anecdotes** were rejected: they can't separate the tool
  from the developer.
- **Lines of code alone** were rejected: shorter is not faster to write,
  and it ignores the time spent satisfying the borrow checker, which is
  the point of the comparison.
- **One person doing each task in both arms** was rejected: the second
  arm benefits from the first. Agents in fresh sessions don't carry the
  solution over, so they may do both.
- **Comparing with plain Go** is not the question. Plain Go is the speed
  GoPatina hopes to keep; the question is the cost of getting Rust's
  output. Plain Go times may be recorded for reference later.

## Prior art

- Controlled experiments on programming languages and type systems (for
  example, Prechelt's comparison of seven languages, 2000, and the static
  versus dynamic typing studies by Hanenberg and others) use specified
  tasks, acceptance tests and time to a working solution. They report
  large differences between individual developers, which is why this
  design compares within tasks and reports intervals.
- Their common weakness is small, student-only samples, which is why
  experience is recorded and reported here.

## Decisions

The project lead answered the three open questions: "Yes to all three."
(Peter, 2026-11-23).

- **Agent runs.** 10 runs per task and arm, fixed in the harness after a
  pilot of one run per task and arm (12 runs). The pilot is published and
  not counted. The agents' turn limit is set from the pilot, before any
  counted run.
- **Time cap.** One cap for every task: 2 hours for people, and a fixed
  turn limit for agents. No per-task caps.
- **Inviting people.** First a sign-up page on gopatina.org and a blog
  post. Posts on Go and Rust forums (r/golang, r/rust, the Go forum) come
  later, after the agent results, and each needs the project lead's yes
  before it goes out.
- **Pacing.** Agent runs use the project lead's Claude quota, which is
  limited, so the 120 counted runs are spread over several days in small
  daily batches.

## Unresolved questions

None.

## Future possibilities

- Tasks that use more of Go as the subset grows (goroutines, maps).
- A maintenance task: changing an existing program to a new specification,
  which may matter more than writing one from scratch.
- Plain Go as a third arm, for reference.
