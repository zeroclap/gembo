import copy
from datetime import datetime, timedelta, timezone
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

from jsonschema.exceptions import ValidationError

import contract


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name).resolve()
        policy = json.loads((contract.ROOT / contract.POLICY).read_text())
        for name in policy["trusted_paths"] + [policy["input_path"]]:
            target = root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(contract.ROOT / name, target)
        root_patch = patch.object(contract, "ROOT", root)
        root_patch.start()
        self.addCleanup(root_patch.stop)
        head_patch = patch.object(contract, "git_head", return_value="a" * 40)
        self.head = head_patch.start()
        self.addCleanup(head_patch.stop)
        self.task_name = ".artifacts/test/task.json"
        self.result_name = ".artifacts/test/result.json"
        contract.prepare(self.task_name, self.result_name)

    def save(self, name, value):
        contract.local_path(name).write_text(json.dumps(value))

    def change_source(self, change):
        task = contract.read_json(self.task_name)
        name = task["inputs"][0]["path"]
        source = contract.read_json(name)
        change(source)
        self.save(name, source)
        task["inputs"][0]["sha256"] = contract.digest(contract.local_path(name))
        self.save(self.task_name, task)

    def run_review(self):
        return contract.review(self.task_name, ["python3", "scripts/verification/contract.py", "review", "--task", self.task_name])

    def test_red_evidence_review_succeeds_without_claiming_release(self):
        result = self.run_review()
        self.assertEqual(result["status"], "succeeded")
        self.assertEqual(result["evidence_verdict"], "gaps")
        self.assertFalse(result["release_eligible"])
        self.assertEqual(result["evidence"]["counts"], {"pass": 11, "fail": 4, "skip": 1, "not-run": 5})
        go_test = next(c for c in result["evidence"]["checks"] if c["name"] == "go-test")
        self.assertEqual(go_test["tests"]["cases"], {"pass": 342, "skip": 2})
        self.assertEqual(len(go_test["tests"]["case_skips"]), 2)
        self.assertIsNone(result["usage"]["cost_usd"])
        self.assertEqual(contract.validate_result(self.task_name, result), result)

    def test_false_pass_or_altered_identity_command_and_risks_rejected(self):
        original = self.run_review()
        changes = [
            lambda r: r.update(evidence_verdict="complete"),
            lambda r: r.update(release_eligible=True),
            lambda r: r.update(task_id="different"),
            lambda r: r.update(head_commit="b" * 40),
            lambda r: r.update(unresolved_risks=[]),
            lambda r: r["evidence"]["counts"].update(fail=0),
            lambda r: r["evidence"]["checks"][0].update(command=["true"]),
        ]
        for change in changes:
            with self.subTest(change=change):
                result = copy.deepcopy(original)
                change(result)
                with self.assertRaises((ValueError, ValidationError)):
                    contract.validate_result(self.task_name, result)

    def test_missing_duplicate_and_unknown_checks_rejected(self):
        original = contract.read_json("docs/reports/d02-baseline.json")
        for checks in (original["checks"][:-1], original["checks"] + [original["checks"][0]],
                       [{**original["checks"][0], "name": "unknown"}] + original["checks"][1:]):
            self.change_source(lambda source: source.update(checks=checks))
            with self.assertRaisesRegex(ValueError, "check"):
                self.run_review()
        self.assertFalse(contract.local_path(self.result_name).exists())

    def test_empty_or_hidden_skips_and_inconsistent_counts_rejected(self):
        original = contract.read_json("docs/reports/d02-baseline.json")
        for mode in ("hide-skip", "empty", "counts"):
            def change(source):
                source.update(copy.deepcopy(original))
                check = next(c for c in source["checks"] if c["name"] == "go-test")
                if mode == "hide-skip":
                    check["status"] = "pass"
                    source["counts"].update({"pass": 12, "skip": 0})
                elif mode == "empty":
                    check["tests"]["cases"] = {}
                else:
                    source["counts"]["pass"] += 1
            self.change_source(change)
            with self.subTest(mode=mode), self.assertRaises(ValueError):
                self.run_review()

    def test_hash_drift_stale_revision_and_expired_run_stop(self):
        self.head.return_value = "b" * 40
        with self.assertRaisesRegex(ValueError, "base commit"):
            self.run_review()
        self.head.return_value = "a" * 40
        task = contract.read_json(self.task_name)
        task["deadline"] = (datetime.now(timezone.utc) - timedelta(seconds=1)).isoformat()
        self.save(self.task_name, task)
        with self.assertRaisesRegex(ValueError, "expired"):
            self.run_review()
        contract.local_path("docs/reports/d02-baseline.json").write_text("{}")
        with self.assertRaisesRegex(ValueError, "hash mismatch"):
            self.run_review()
        self.assertFalse(contract.local_path(self.result_name).exists())

    def test_unknown_fields_commands_scope_and_budget_are_rejected(self):
        original = contract.read_json(self.task_name)
        changes = [lambda t: t.update(extra="ignored?"),
                   lambda t: t.update(allowed_command_ids=["shell"]),
                   lambda t: t["scope"].update(output_path="backend/changed.go"),
                   lambda t: t["scope"].update(output_path=".artifacts/../secret"),
                   lambda t: t["budgets"].update(network_requests=1),
                   lambda t: t.update(publisher_target="main")]
        for change in changes:
            task = copy.deepcopy(original)
            change(task)
            self.save(self.task_name, task)
            with self.subTest(change=change), self.assertRaises((ValueError, ValidationError)):
                self.run_review()

    def test_symlinks_oversize_and_duplicate_json_fields_rejected(self):
        source = contract.local_path("docs/reports/d02-baseline.json")
        source.unlink()
        source.symlink_to(contract.local_path(self.task_name))
        with self.assertRaisesRegex(ValueError, "symlink"):
            contract.read_json("docs/reports/d02-baseline.json")
        source.unlink()
        source.write_text(" " * (contract.MAX_BYTES + 1))
        with self.assertRaisesRegex(ValueError, "oversized"):
            contract.read_json("docs/reports/d02-baseline.json")
        source.write_text('{"x":1,"x":2}')
        with self.assertRaisesRegex(ValueError, "duplicate"):
            contract.read_json("docs/reports/d02-baseline.json")

    def test_injected_commands_remain_data_and_no_subprocess_runs(self):
        marker = contract.ROOT / "injected"
        def change(source):
            source["checks"][0]["command"] = ["sh", "-c", f"touch {marker}"]
        self.change_source(change)
        with patch.object(contract.subprocess, "check_output", side_effect=AssertionError("unexpected process")):
            result = self.run_review()
        self.assertIn("touch", result["evidence"]["checks"][0]["command"][2])
        self.assertFalse(marker.exists())

    def test_existing_output_and_over_budget_result_are_rejected(self):
        result = self.run_review()
        previous = contract.local_path(self.result_name).read_bytes()
        with self.assertRaises(FileExistsError):
            self.run_review()
        self.assertEqual(contract.local_path(self.result_name).read_bytes(), previous)
        result["usage"]["wall_seconds"] = 1801
        with self.assertRaisesRegex(ValueError, "budget"):
            contract.validate_result(self.task_name, result)

    def test_archived_validation_does_not_admit_a_new_stale_run(self):
        result = self.run_review()
        self.head.return_value = "b" * 40
        self.assertEqual(contract.validate_result(self.task_name, result), result)
        with self.assertRaisesRegex(ValueError, "base commit"):
            self.run_review()

    def test_executable_rejects_invalid_task_and_usage_without_success_output(self):
        task = contract.read_json(self.task_name)
        task["unexpected"] = "must be rejected"
        self.save(self.task_name, task)
        executable = [sys.executable, str(contract.ROOT / "scripts/verification/contract.py")]
        invalid = subprocess.run(executable + ["review", "--task", self.task_name],
                                 capture_output=True, text=True, timeout=10)
        self.assertEqual(invalid.returncode, 1)
        self.assertEqual(invalid.stdout, "")
        self.assertIn("verification contract:", invalid.stderr)
        self.assertNotIn("Traceback", invalid.stderr)
        usage = subprocess.run(executable + ["unknown"], capture_output=True, text=True, timeout=10)
        self.assertEqual(usage.returncode, 2)
        self.assertEqual(usage.stdout, "")
        self.assertFalse(contract.local_path(self.result_name).exists())


if __name__ == "__main__":
    unittest.main()
