#!/usr/bin/env python3
"""D03 manual evidence contract; never execute commands found in input records."""

import argparse
from collections import Counter
from datetime import datetime, timedelta, timezone
import hashlib
import json
from pathlib import Path
import platform
import re
import subprocess
import sys
import time
import uuid

from jsonschema import Draft202012Validator
from jsonschema.exceptions import SchemaError, ValidationError


ROOT = Path(__file__).resolve().parents[2]
MAX_BYTES = 1048576
POLICY = "agents/policies/verification.json"
STATUSES = ("pass", "fail", "skip", "not-run")
OUTCOME = "D02 evidence review completed; imported outcomes are not new application checks."
RISKS = [
    "D02 commands were not rerun; all application outcomes are historical imported evidence.",
    "Raw D02 logs are local-only and absent from fresh checkouts; their truth is not attested here.",
    "Independent D01/D02 review and Phase 0 acceptance remain open.",
    "C01/C02/D07 application CLI inventory, executable and headless journey remain undelivered.",
    "D04/D05/D06/C06/D12/D13 retain the image, store, lint, UI and live-qualification gaps.",
    "Conversational model usage/cost/tool calls are unavailable; hosted enforcement is D10 work.",
]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def local_path(name):
    require(isinstance(name, str) and re.fullmatch(r"[A-Za-z0-9_./-]+", name), "invalid path")
    relative = Path(name)
    require(not relative.is_absolute() and ".." not in relative.parts, "unsafe path")
    target = ROOT / relative
    require(not any(p.is_symlink() for p in (target, *target.parents)), "symlink path")
    return target


def digest(path):
    require(path.stat().st_size <= MAX_BYTES, "oversized input")
    return hashlib.sha256(path.read_bytes()).hexdigest()


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, f"duplicate JSON field: {key}")
        result[key] = value
    return result


def read_json(name):
    path = local_path(name)
    require(path.stat().st_size <= MAX_BYTES, "oversized input")
    return json.loads(path.read_text(), object_pairs_hook=unique_object)


def validate_schema(kind, value):
    schema = read_json(f"agents/schemas/verification-{kind}.schema.json")
    Draft202012Validator.check_schema(schema)
    # Schemas use internal structure only; no remote $refs or network resolution.
    Draft202012Validator(schema).validate(value)


def git_head():
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, timeout=10).decode().strip()


def timestamp(value):
    parsed = datetime.fromisoformat(value)
    require(parsed.utcoffset() == timedelta(0), "timestamp must be UTC")
    return parsed


def output_path(name, policy):
    require(name == policy["public_output"] or any(name.startswith(p) for p in policy["output_roots"]),
            "output outside allowed scope")
    return local_path(name)


def write_new(path, value, limit=MAX_BYTES):
    data = json.dumps(value, indent=2) + "\n"
    require(len(data.encode()) <= limit, "oversized output")
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("x") as handle:
        handle.write(data)


def load_task(name):
    task = read_json(name)
    validate_schema("task", task)
    policy = read_json(POLICY)
    expected_paths = policy["trusted_paths"]
    require([r["path"] for r in task["trusted_bundle"]] == expected_paths, "trusted bundle paths mismatch")
    require([r["path"] for r in task["inputs"]] == [policy["input_path"]], "input scope mismatch")
    for ref in task["trusted_bundle"] + task["inputs"]:
        require(digest(local_path(ref["path"])) == ref["sha256"], f"hash mismatch: {ref['path']}")
    require(set(task["scope"]["read_paths"]) == set(expected_paths + [policy["input_path"]]),
            "read scope mismatch")
    require(task["budgets"] == policy["budgets"], "budget mismatch")
    output_path(task["scope"]["output_path"], policy)
    timestamp(task["deadline"])
    return task, policy


def imported_evidence(task, policy):
    baseline = read_json(task["inputs"][0]["path"])
    require(baseline["schema_version"] == 1, "unsupported baseline schema")
    original = baseline["checks"]
    names = [check["name"] for check in original]
    require(len(names) == len(set(names)) and set(names) == set(policy["required_checks"]),
            "missing, duplicate or unknown check")
    counts = {status: sum(c["status"] == status for c in original) for status in STATUSES}
    require(sum(counts.values()) == len(original), "unknown check status")
    require(Counter(baseline["counts"]) == Counter(counts), "baseline counts mismatch")
    checks = []
    for check in original:
        status, code = check["status"], check.get("exit_code")
        require(status not in {"pass", "skip"} or code in {None, 0}, "nonzero check claimed pass/skip")
        require(not check.get("timeout") or status == "fail", "timeout claimed success")
        tests = check.get("tests")
        if check["name"] == "go-test":
            require(tests and sum(tests["cases"].values()) > 0, "empty Go case evidence")
            require(len(tests["case_skips"]) == tests["cases"].get("skip", 0), "case skips mismatch")
            expected = "fail" if code != 0 or tests["cases"].get("fail", 0) else (
                "skip" if tests["cases"].get("skip", 0) else "pass")
            require(status == expected, "Go test status hides failed/skipped cases")
        if status == "not-run":
            require(bool(check.get("reason")), "not-run reason missing")
        checks.append({
            "name": check["name"], "status": status, "exit_code": code,
            "command": check.get("command", check.get("command_template", [])),
            "command_kind": "exact-import" if "command" in check else (
                "template-import" if "command_template" in check else "not-recorded"),
            "reason": check.get("reason"),
            "tests": {key: tests[key] for key in ("cases", "packages", "case_skips")} if tests else None,
        })
    return {
        "classification": "imported-only", "input": task["inputs"][0],
        "source_run_id": baseline["run_id"], "source_revision": baseline["revision"],
        "source_snapshot_sha256": baseline["source_snapshot_sha256"],
        "counts": counts, "checks": checks,
    }


def validate_result(task_name, result):
    task, policy = load_task(task_name)
    validate_schema("result", result)
    require(result["task_sha256"] == digest(local_path(task_name)), "task hash mismatch")
    for key in ("task_id", "run_id", "base_commit"):
        require(result[key] == task[key], f"result {key} mismatch")
    require(result["head_commit"] == task["base_commit"], "head mismatch")
    evidence = imported_evidence(task, policy)
    require(result["evidence"] == evidence, "result differs from imported evidence")
    verdict = "complete" if evidence["counts"]["pass"] == len(evidence["checks"]) else "gaps"
    require(result["evidence_verdict"] == verdict, "false evidence verdict")
    require(result["status"] == "succeeded" and result["outcome"] == OUTCOME, "invalid review completion")
    require(result["unresolved_risks"] == RISKS, "unresolved risks were removed or changed")
    execution = result["execution"]
    start, end = timestamp(execution["started_at"]), timestamp(execution["ended_at"])
    require(start <= end <= timestamp(task["deadline"]), "result timestamps outside deadline")
    require((end - start).total_seconds() <= task["budgets"]["max_wall_seconds"], "wall budget exceeded")
    require(result["usage"]["wall_seconds"] <= task["budgets"]["max_wall_seconds"], "usage budget exceeded")
    return result


def prepare(task_name, result_name):
    policy = read_json(POLICY)
    output_path(result_name, policy)
    require(task_name == "docs/reports/d03-verification-task.json" or task_name.startswith(".artifacts/"),
            "task destination outside rehearsal scope")
    require(task_name != result_name, "task/result destinations must differ")
    run_id = "d03-" + datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ-").lower() + uuid.uuid4().hex[:8]
    refs = [{"path": name, "sha256": digest(local_path(name))} for name in policy["trusted_paths"]]
    task = {
        "schema_version": "1.0.0", "task_id": "d03-review-d02", "run_id": run_id,
        "repository": {"id": 1186938709, "full_name": "zeroclap/gembo"}, "issue_url": None,
        "role": "verification", "role_version": "1.0.0", "tool_version": "verification-contract/1.0.0",
        "authority": "contract-rehearsal", "base_commit": git_head(),
        "objective": "Review all recorded D02 checks without rerunning application or exchange commands.",
        "acceptance_criteria": ["Retain all 21 imported checks and original provenance.",
                                "Preserve failures, skips, not-run checks and explicit unknowns.",
                                "Validate the task/result contract without granting operating authority."],
        "risk": "R0", "environment": "evidence-only", "trusted_bundle": refs,
        "inputs": [{"path": policy["input_path"], "sha256": digest(local_path(policy["input_path"]))}],
        "scope": {"read_paths": policy["trusted_paths"] + [policy["input_path"]], "output_path": result_name},
        "allowed_command_ids": ["review-d02"], "budgets": policy["budgets"],
        "deadline": (datetime.now(timezone.utc) + timedelta(seconds=1800)).isoformat(),
        "approval_refs": [], "publisher_target": None, "gembo_cli": policy["gembo_cli"],
    }
    validate_schema("task", task)
    write_new(local_path(task_name), task)


def review(task_name, argv):
    started_at, clock_start = datetime.now(timezone.utc), time.monotonic()
    task, policy = load_task(task_name)
    require(git_head() == task["base_commit"], "base commit changed")
    require(started_at <= timestamp(task["deadline"]), "deadline expired")
    evidence = imported_evidence(task, policy)
    result = {
        "schema_version": "1.0.0", "task_id": task["task_id"], "run_id": task["run_id"],
        "task_sha256": digest(local_path(task_name)), "base_commit": task["base_commit"],
        "head_commit": git_head(), "patch_sha256": None, "changed_paths": [],
        "status": "succeeded", "outcome": OUTCOME, "evidence": evidence,
        "evidence_verdict": "complete" if evidence["counts"]["pass"] == len(evidence["checks"]) else "gaps",
        "release_eligible": False, "unresolved_risks": RISKS, "requested_next_action": "owner-review",
        "usage": {"wall_seconds": round(time.monotonic() - clock_start, 6), "model_tokens": None,
                  "cost_usd": None, "tool_calls": None,
                  "unavailable_reason": "Manual session usage is not metered; checker makes no model/API calls."},
        "execution": {"command_id": "review-d02", "argv": argv, "python_version": platform.python_version(),
                      "started_at": started_at.isoformat(), "ended_at": datetime.now(timezone.utc).isoformat()},
        "cleanup": {"status": "no-resources-created", "detail": "Only the scoped result is written; no child services, containers, network or temporary workspace."},
    }
    validate_result(task_name, result)
    write_new(output_path(task["scope"]["output_path"], policy), result, task["budgets"]["max_output_bytes"])
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("prepare", "review", "validate"):
        command = sub.add_parser(name)
        command.add_argument("--task", required=True, help="repository-relative task path")
        if name == "prepare":
            command.add_argument("--output", required=True, help="new repository-relative result path")
        if name == "validate":
            command.add_argument("--result", required=True)
    args = parser.parse_args()
    try:
        if args.command == "prepare":
            prepare(args.task, args.output)
        elif args.command == "review":
            interpreter = str(Path(sys.executable).relative_to(ROOT)) if Path(sys.executable).is_relative_to(ROOT) else sys.executable
            review(args.task, [interpreter, "scripts/verification/contract.py", *sys.argv[1:]])
        else:
            task, _ = load_task(args.task)
            require(args.result == task["scope"]["output_path"], "result path outside task scope")
            validate_result(args.task, read_json(args.result))
    except (ValueError, OSError, KeyError, TypeError, SchemaError, ValidationError, subprocess.SubprocessError) as error:
        print(f"verification contract: {error}", file=sys.stderr)
        return 1
    print(f"{args.command}: valid contract rehearsal; no application checks executed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
