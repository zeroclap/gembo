# Reproducing the D02 baseline

Run from the repository root with Git, Docker Engine/BuildKit and Python **3.10 or newer**. Host Go, Node, npm, Redis, PostgreSQL and exchange credentials are unnecessary. Python and Docker are orchestration prerequisites; their observed versions/platform are evidence, not application toolchain pins. Linux containers are supported on Linux and Docker-enabled macOS; the first recorded platform is Linux arm64 on macOS arm64. Linux amd64 image manifests exist but require a separate execution result.

```bash
make bootstrap
make test-verification
make baseline
```

`bootstrap` builds the tooling image and verifies its actual executable versions. `baseline` includes the same bootstrap, installs locked dependencies into a new temporary workspace, then runs every implemented check below. Both commands write a new evidence directory under ignored `.artifacts/`; they never start application services. For a chosen new output directory:

```bash
python3 scripts/verification/baseline.py --output /tmp/gembo-baseline-example
```

The path must not already exist. A baseline exits **0** only if every selected check passes; **1** means a failure, skip, missing dependency, not-run check or interruption remains. A complete red baseline is useful D02 evidence. It is not a release gate pass. `make baseline` is therefore currently expected to return nonzero for known gaps. There is no automatic retry or failure suppression.

## Tool and dependency pins

[toolchain.json](../tools/verification/toolchain.json) records exact Go, Node, npm and golangci-lint versions. The [tooling Dockerfile](../tools/verification/Dockerfile) pins all three upstream image indexes by SHA-256, includes the Node distribution's npm, and performs no unpinned package installation. A runtime check fails on version drift. Go stays on the repository's declared 1.25 language line with its pinned patch; this baseline does not establish a supported production runtime or a security qualification.

Application dependencies remain unchanged: Go modules use committed `go.mod`/`go.sum`, `GOTOOLCHAIN=local`, `GOWORK=off` and `-mod=readonly`; frontend uses `npm ci` with the committed npm lockfile. Go checks share `-trimpath` with the service Make targets so they reuse one compilation profile. ESLint, TypeScript and Next are pinned by that lockfile. Original and post-run lock hashes are compared. No tidy, dependency upgrade or lock regeneration is part of this command.

Upstream pin provenance: [Go downloads](https://go.dev/dl/), [Node release index](https://nodejs.org/dist/index.json), [official Go image source](https://github.com/docker-library/golang), [official Node image source](https://github.com/nodejs/docker-node), [golangci-lint release](https://github.com/golangci/golangci-lint/releases/tag/v2.15.0). Registry digests were resolved on 28 October 2026. Future pin changes require another measured baseline.

## Isolation, resources and cleanup

The runner exports only Git-tracked `backend/` and `frontend/` files, including current tracked edits. It omits all `.env*`, `.npmrc`, `.netrc`, dependency directories, build output and Git metadata; source symlinks are rejected. Untracked application files are omitted. To test new application files after D02, stage their intended paths first or implement a reviewed snapshot extension; this runner does not stage files itself. The original checkout and developer home are never mounted into a container. Source and lock hashes identify the exact exported content even before commit.

Each run gets fresh source, home, npm, Go module/build and lint directories. Each check gets a separate container using the caller's UID/GID (non-root `501:20` in the recorded run), with no published ports, no Docker socket, dropped capabilities, 2 CPUs, 10 GiB memory and a 512-process limit. Allow Docker RAM above the 10 GiB per-container ceiling. Go uses `-p=1`, `GOMAXPROCS=2`, `GOMEMLIMIT=6GiB` and `GOGC=50` to contain the large CCXT compilation; the first 6 GiB container attempt was killed and is retained in the handoff. The Go limit is per process and soft; the container limit remains the hard ceiling. Only dependency downloads use the Docker bridge network. Builds, lint, vet and tests use `--network none`: their loopback is private, so the legacy `localhost:6379` tests cannot contact a developer's Redis DB 15. No database service is supplied in D02. This reports the integration gap; D05 must replace it with mandatory disposable stores.

Inherited host variables, npm configuration and credentials are not forwarded. Explicit container variables are in each recorded command. Image builds receive only the tooling Dockerfile as context. Dependency installation executes normal package installation scripts inside the disposable container; it does not run application processes. Offline frontend builds may reveal an external font download dependency; such failures remain in evidence.

The runner records per-check cgroup v2 peak memory, including tooling overhead. `null` plus a reason means measurement was unavailable; the memory ceiling is never presented as measured usage. A source snapshot and dependency caches are removed on exit. Containers use `--rm`; named-container cleanup also runs on timeout/interruption. A host crash or SIGKILL can leave a container: inspect its recorded `d02-<run-id>-<check>` name before removing that exact container. Do not use global Docker prune. Tool images and Docker build caches remain reusable; only a reviewed cleanup should delete them.

## Checks and evidence

| Check | Command in container | Scope |
|---|---|---|
| Dependencies | `go mod download`; `npm ci --no-audit --no-fund` | Clean installation; network confined to this step |
| Formatting | `gofmt -l <all tracked Go sources>` | Nonempty output is a failure |
| Service build | `make build` in `backend/` | All five current service binaries |
| Utility build | `go build ./cmd/...` in `backend/` | Includes all three specialist utilities |
| Go tests | `make test TEST_FLAGS='-json -count=1 -timeout=120s'` | Same package set as `make test`, structured evidence and fresh execution |
| Go analysis | `make vet`; `make lint`; `go mod verify` | Existing checks and downloaded module integrity |
| Frontend | `npm run lint`; `npm run build`; `./node_modules/.bin/tsc --noEmit --incremental false` | Existing scripts plus explicit type analysis |
| Locks | SHA-256 comparison | Installation/checks must preserve the committed dependency graph |

The backend's optional `TEST_FLAGS` leaves plain `make test` behavior intact. JSON terminal test events are counted separately from package events. Individual skip identities are retained; an exit-zero test command containing skipped tests is classified `skip`. A successful process without any terminal test events fails evidence validation. Failed prerequisite installation makes its dependent checks `not-run`, not passed.

`baseline.json` contains schema version, run/revision/branch, dirty flag, exported file and lock hashes, tooling image ID, host platform, exact commands, start/end times, durations, exit statuses, memory, per-case test totals, skip identities, and explicit not-run reasons. `baseline.md` indexes every check log. CLI/version/protocol evidence is explicitly unavailable until C01/C02/D07. Container packaging (D04), disposable-store integration (D05), frontend test tooling (D06/C06), and live qualification (D12/D13) are classified separately. Mutating developer commands such as tidy/clean/run/Compose are outside verification.

`make test-verification` tests export isolation, source fingerprinting, symlink rejection, container boundaries, failure/timeout evidence, skip accounting, report output, and Make flag compatibility. These harness tests do not replace application tests.

Generated evidence stays in ignored `.artifacts/`. Keep only a curated handoff and compact baseline JSON in `docs/reports/`; retain raw logs, per-file hash inventories, intermediate attempts, duplicate indexes and review manifests locally under `.artifacts/`. The compact summary records outcomes, failures/skips, tool/lock/source hashes, commands or declared templates, timings and resource measurements. Identify the local archive and full-baseline hash without making those local files a checkout dependency; a fresh checkout can regenerate diagnostics with `make baseline`. Never publish temporary workspaces, dependency caches, binaries, `.next`, npm debug logs or private payloads. See the [D02 handoff](reports/d02-reproducible-baseline.md) and [compact baseline](reports/d02-baseline.json) for the selected run, remaining defects and proposed publication set.
