#!/usr/bin/env python3
"""Credential-free D02 baseline. Python 3.10+, Git and Docker are host prerequisites."""

import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tempfile
import time
import uuid


ROOT = Path(__file__).resolve().parents[2]
TOOLS = ROOT / "tools/verification"
LOCKS = ("backend/go.mod", "backend/go.sum", "frontend/package-lock.json")
EXCLUDED_PARTS = {"node_modules", ".next", ".git", "bin", ".npmrc", ".netrc"}


def now():
    return datetime.now(timezone.utc).isoformat()


def sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def allowed_source(name):
    path = Path(name)
    return (
        path.parts[0] in {"backend", "frontend"}
        and not path.is_absolute()
        and ".." not in path.parts
        and not any(p in EXCLUDED_PARTS or p.startswith(".env") for p in path.parts)
    )


def snapshot(root, destination, names):
    """Copy only versioned application files, never follow source symlinks."""
    hashes = {}
    for name in sorted(names):
        if not allowed_source(name):
            continue
        source = root / name
        if source.is_symlink() or any((root / p).is_symlink() for p in Path(name).parents):
            raise ValueError(f"source symlink is not permitted: {name}")
        if not source.exists():  # A tracked deletion is part of the snapshot too.
            continue
        target = destination / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source, target)
        hashes[name] = sha256(target)
    fingerprint = hashlib.sha256(json.dumps(hashes, sort_keys=True).encode()).hexdigest()
    return hashes, fingerprint


def parse_go_events(text):
    """Count terminal case/package events separately; retain every explicit skip."""
    cases, packages, skips = Counter(), Counter(), []
    for line in text.splitlines():
        if not line.startswith("{"):
            continue
        event = json.loads(line)
        action = event.get("Action")
        if action not in {"pass", "fail", "skip"}:
            continue
        (cases if event.get("Test") else packages)[action] += 1
        if action == "skip":
            skips.append({k: event[k] for k in ("Package", "Test") if k in event})
    return {"cases": dict(cases), "packages": dict(packages), "skips": skips}


def classify(exit_code, events=None):
    if exit_code != 0:
        return "fail"
    if events is not None:
        if not events["cases"]:
            return "fail"
        if events["cases"].get("skip", 0):
            return "skip"
    return "pass"


def container_command(image, name, workspace, state, directory, argv, network="none"):
    env = {
        "HOME": "/state/home", "GOPATH": "/state/go", "GOCACHE": "/state/go-build",
        "GOLANGCI_LINT_CACHE": "/state/lint", "GOTOOLCHAIN": "local", "GOWORK": "off",
        "GOFLAGS": "-mod=readonly -p=1 -trimpath", "GOMAXPROCS": "2",
        "GOMEMLIMIT": "6GiB", "GOGC": "50", "CI": "1",
        "NEXT_TELEMETRY_DISABLED": "1", "npm_config_cache": "/state/npm",
        "npm_config_userconfig": "/state/npm-userconfig", "npm_config_globalconfig": "/state/npm-globalconfig",
        "npm_config_registry": "https://registry.npmjs.org/",
    }
    command = [
        "docker", "run", "--rm", "--init", "--name", name,
        "--network", network, "--cap-drop=ALL", "--security-opt=no-new-privileges",
        "--memory=10g", "--cpus=2", "--pids-limit=512",
        "--user", f"{os.getuid()}:{os.getgid()}",
        "--mount", f"type=bind,source={workspace},target=/workspace",
        "--mount", f"type=bind,source={state},target=/state",
        "--workdir", f"/workspace/{directory}",
    ]
    for key, value in env.items():
        command.extend(["--env", f"{key}={value}"])
    # No host environment forwarding, Docker socket, published ports, or host network.
    # Each check has its own cgroup. Read its peak without replacing the check's exit code.
    measured = ["sh", "-c", '"$@"; result=$?; cat /sys/fs/cgroup/memory.peak > /state/metrics/' + name + '; exit "$result"', "--", *argv]
    return command + [image] + measured


class Run:
    def __init__(self, output, run_id):
        self.output, self.run_id, self.checks = output, run_id, []
        self.active_container = None

    def check(self, name, command, timeout=600, go_json=False, empty_output=False, **metadata):
        print(f"{name}: running", flush=True)
        log = self.output / f"{name}.log"
        start, started_at, timed_out = time.monotonic(), now(), False
        try:
            with log.open("w") as handle:
                proc = subprocess.Popen(command, stdout=handle, stderr=subprocess.STDOUT)
                try:
                    code = proc.wait(timeout=timeout)
                except subprocess.TimeoutExpired:
                    timed_out = True
                    proc.kill()
                    proc.wait()
                    code = 124
                except BaseException:
                    proc.kill()
                    proc.wait()
                    raise
        except OSError as error:
            log.write_text(f"{type(error).__name__}: {error}\n")
            code = 127
        finally:
            if self.active_container:
                subprocess.run(["docker", "rm", "-f", self.active_container],
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
                self.active_container = None
        events = parse_go_events(log.read_text(errors="replace")) if go_json else None
        result = {
            "name": name, "status": classify(code, events), "exit_code": code,
            "started_at": started_at, "ended_at": now(),
            "duration_seconds": round(time.monotonic() - start, 3), "timeout": timed_out,
            "command": command, "log": log.name, "log_sha256": sha256(log), **metadata,
        }
        if empty_output and code == 0 and log.stat().st_size:
            result["status"] = "fail"
            result["reason"] = "command reported findings despite a zero exit code"
        if events is not None:
            result["tests"] = events
        self.checks.append(result)
        print(f"{name}: {result['status']} (exit {code})", flush=True)
        return result

    def not_run(self, name, reason, command):
        self.checks.append({"name": name, "status": "not-run", "reason": reason, "command": command})
        print(f"{name}: not-run ({reason})", flush=True)

    def container(self, name, image, workspace, state, directory, argv, network="none", **kwargs):
        self.active_container = f"{self.run_id}-{name}"
        command = container_command(image, self.active_container, workspace, state, directory, argv, network)
        metric = state / "metrics" / self.active_container
        result = self.check(name, command, network=network, cwd=directory, **kwargs)
        raw_peak = metric.read_text().strip() if metric.exists() else ""
        result["cgroup_peak_memory_bytes"] = int(raw_peak) if raw_peak.isdigit() else None
        if not raw_peak.isdigit():
            result["memory_measurement_gap"] = "cgroup v2 memory.peak unavailable or check interrupted"
        return result


def write_report(output, report):
    report["counts"] = dict(Counter(c["status"] for c in report["checks"]))
    report["ended_at"] = now()
    (output / "baseline.json").write_text(json.dumps(report, indent=2) + "\n")
    rows = ["# D02 baseline run", "", f"Source: `{report['revision']}`.", "",
            "| Check | Status | Evidence |", "|---|---|---|"]
    rows += [f"| {c['name']} | {c['status']} | " +
             (f"[{c['log']}]({c['log']})" if "log" in c else c.get("reason", "recorded in baseline.json")) + " |"
             for c in report["checks"]]
    rows += ["", "See [baseline.json](baseline.json) for exact commands, hashes, timings and skips.", ""]
    (output / "baseline.md").write_text("\n".join(rows))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bootstrap-only", action="store_true")
    parser.add_argument("--output", type=Path, help="new evidence directory (default: .artifacts/<run-id>)")
    args = parser.parse_args()
    run_id = "d02-" + datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + uuid.uuid4().hex[:8]
    output = (args.output or ROOT / ".artifacts" / run_id).resolve()
    output.mkdir(parents=True, exist_ok=False)
    run = Run(output, run_id)
    manifest = json.loads((TOOLS / "toolchain.json").read_text())
    image_tag = "gembo-verification:" + sha256(TOOLS / "Dockerfile")[:16]
    git = lambda *a: subprocess.check_output(["git", "-C", str(ROOT), *a]).decode().strip()
    report = {
        "schema_version": 1, "run_id": run_id, "started_at": now(),
        "revision": git("rev-parse", "HEAD"), "branch": git("branch", "--show-current"),
        "dirty": bool(git("status", "--porcelain")),
        "host": {"os": platform.system(), "arch": platform.machine(), "python": platform.python_version()},
        "toolchain": manifest, "checks": run.checks,
        "runner_sha256": sha256(Path(__file__)), "dockerfile_sha256": sha256(TOOLS / "Dockerfile"),
        "frontend_locked_tools": {key: json.loads((ROOT / "frontend/package-lock.json").read_text())["packages"]["node_modules/" + key]["version"] for key in ("eslint", "typescript", "next", "react")},
        "profile": "synthetic-only; no env files/home credentials; isolated loopback; no DB services",
        "limits": {"memory_bytes": 10737418240, "cpus": 2, "pids": 512, "go_memory_limit": "6GiB", "go_gc_percent": 50, "go_build_parallelism": 1},
        "memory_measurement": "per-check cgroup v2 memory.peak, including tooling overhead; null means unavailable",
        "cli": {"status": "not-implemented", "next_packages": ["C01", "C02", "D07"]},
    }
    try:
        run.check("docker-version", ["docker", "version", "--format", "{{json .}}"], timeout=30)
        bootstrap = run.check("bootstrap", ["docker", "build", "--pull", "--progress=plain", "-t", image_tag, str(TOOLS)], timeout=1200)
        ready = bootstrap["status"] == "pass"
        if ready:
            image_id = subprocess.check_output(["docker", "image", "inspect", "--format", "{{.Id}}", image_tag]).decode().strip()
            report["image_id"] = image_id
        else:
            image_id = image_tag
        with tempfile.TemporaryDirectory(prefix="gembo-d02-") as temporary:
            work = Path(temporary) / "source"
            state = Path(temporary) / "state"
            work.mkdir()
            (state / "home").mkdir(parents=True)
            (state / "metrics").mkdir()
            names = git("ls-files", "-z").rstrip("\0").split("\0")
            hashes, fingerprint = snapshot(ROOT, work, names)
            report["source_snapshot_sha256"] = fingerprint
            report["source_files_sha256"] = hashes
            report["lock_sha256"] = {name: hashes[name] for name in LOCKS}
            report["test_inventory"] = sorted(name for name in hashes if name.endswith("_test.go"))

            def execute(name, directory, argv, dependencies=(), network="none", **kwargs):
                nonlocal ready
                if not ready or any(c["status"] != "pass" for c in dependencies):
                    run.not_run(name, "toolchain/bootstrap or prerequisite failed", argv)
                    return run.checks[-1]
                return run.container(name, image_id, work, state, directory, argv, network, **kwargs)

            # Verify actual binaries against the manifest, not merely Docker labels.
            version_script = (
                f'test "$(go env GOVERSION)" = "go{manifest["go"]}" && '
                f'test "$(node --version)" = "v{manifest["node"]}" && '
                f'test "$(npm --version)" = "{manifest["npm"]}" && '
                f'golangci-lint version | grep -F "version {manifest["golangci_lint"]} " && '
                'go version && node --version && npm --version && uname -sm'
            )
            versions = execute("tool-versions", "", ["sh", "-c", version_script])
            ready = ready and versions["status"] == "pass"
            if not args.bootstrap_only:
                go_deps = execute("go-dependencies", "backend", ["go", "mod", "download"], network="bridge")
                npm_deps = execute("npm-ci", "frontend", ["npm", "ci", "--no-audit", "--no-fund"], network="bridge")
                go_files = ["/workspace/" + name for name in hashes if name.endswith(".go")]
                execute("go-format", "backend", ["gofmt", "-l", *go_files], empty_output=True)
                execute("go-build", "backend", ["make", "build"], dependencies=(go_deps,))
                execute("go-utilities-build", "backend", ["go", "build", "./cmd/..."], dependencies=(go_deps,))
                execute("go-test", "backend", ["make", "test", "TEST_FLAGS=-json -count=1 -timeout=120s"], dependencies=(go_deps,), go_json=True)
                execute("go-vet", "backend", ["make", "vet"], dependencies=(go_deps,))
                execute("go-lint", "backend", ["make", "lint"], dependencies=(go_deps,))
                execute("frontend-lint", "frontend", ["npm", "run", "lint"], dependencies=(npm_deps,))
                execute("frontend-build", "frontend", ["npm", "run", "build"], dependencies=(npm_deps,))
                execute("frontend-typecheck", "frontend", ["./node_modules/.bin/tsc", "--noEmit", "--incremental", "false"], dependencies=(npm_deps,))
                execute("go-mod-verify", "backend", ["go", "mod", "verify"], dependencies=(go_deps,))
                current_locks = {name: sha256(work / name) for name in LOCKS}
                run.checks.append({"name": "lock-integrity", "status": "pass" if current_locks == report["lock_sha256"] else "fail", "observed_sha256": current_locks})
                run.not_run("application-images", "D04: removed entrypoints/missing runtime targets; source defect retained", ["make", "-C", "backend", "docker-build"])
                run.not_run("database-integration", "D05: disposable Redis/Postgres harness not implemented; legacy WS skips reported in go-test", ["make", "integration", "(planned)"])
                run.not_run("frontend-tests", "no frontend test runner/script; D06/C06", ["npm", "test", "(planned)"])
                run.not_run("cli-e2e", "C01/C02/D07: product CLI and headless journey not implemented", ["make", "e2e-cli", "(planned)"])
                run.not_run("live-exchange-probes", "separate D12/D13 scope; no live network or account operations", ["go", "run", "./cmd/exchange-probe"])
    except (Exception, KeyboardInterrupt) as error:
        report["interruption"] = {"type": type(error).__name__, "detail": str(error)}
        run.not_run("incomplete-run", "runner interrupted; inspect interruption field", [])
    finally:
        write_report(output, report)
        print(f"Evidence: {output}", flush=True)
    return 1 if report.get("interruption") or any(c["status"] != "pass" for c in run.checks) else 0


if __name__ == "__main__":
    raise SystemExit(main())
