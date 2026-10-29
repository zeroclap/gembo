import json
from pathlib import Path
import subprocess
import tempfile
import unittest

import baseline


class SnapshotTests(unittest.TestCase):
    def test_exports_tracked_source_without_secrets_or_caches(self):
        with tempfile.TemporaryDirectory() as temp:
            root, destination = Path(temp) / "repo", Path(temp) / "copy"
            names = ["backend/go.mod", "frontend/app/page.tsx", "backend/.env",
                     "frontend/.env.production", "frontend/.npmrc", "backend/.netrc",
                     "frontend/node_modules/private/data", "backend/bin/api", ".git/config"]
            for name in names:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("synthetic")
            (root / "backend/untracked.key").write_text("must not be copied")
            hashes, fingerprint = baseline.snapshot(root, destination, names)
            self.assertEqual(set(hashes), {"backend/go.mod", "frontend/app/page.tsx"})
            self.assertEqual(len(fingerprint), 64)
            self.assertEqual(sorted(str(p.relative_to(destination)) for p in destination.rglob("*") if p.is_file()), sorted(hashes))

    def test_changes_affect_fingerprint_and_symlinks_are_rejected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "repo"
            (root / "backend").mkdir(parents=True)
            source = root / "backend/go.mod"
            source.write_text("before")
            _, before = baseline.snapshot(root, Path(temp) / "first", ["backend/go.mod"])
            source.write_text("after")
            _, after = baseline.snapshot(root, Path(temp) / "second", ["backend/go.mod"])
            self.assertNotEqual(before, after)
            source.unlink()
            source.symlink_to(Path(temp) / "private")
            with self.assertRaisesRegex(ValueError, "symlink"):
                baseline.snapshot(root, Path(temp) / "third", ["backend/go.mod"])


class EvidenceTests(unittest.TestCase):
    def test_failed_tests_and_skips_are_not_green(self):
        events = [
            {"Action": "pass", "Package": "unit", "Test": "TestOK"},
            {"Action": "pass", "Package": "unit"},
            {"Action": "skip", "Package": "api", "Test": "TestRedis"},
            {"Action": "pass", "Package": "api"},
        ]
        parsed = baseline.parse_go_events("make output\n" + "\n".join(map(json.dumps, events)))
        self.assertEqual(parsed["cases"], {"pass": 1, "skip": 1})
        self.assertEqual(parsed["packages"], {"pass": 2})
        self.assertEqual(parsed["skips"], [{"Package": "api", "Test": "TestRedis"}])
        self.assertEqual(baseline.classify(0, parsed), "skip")
        self.assertEqual(baseline.classify(1, parsed), "fail")
        self.assertEqual(baseline.classify(0, baseline.parse_go_events("no tests")), "fail")

    def test_summary_retains_fail_skip_and_not_run(self):
        with tempfile.TemporaryDirectory() as temp:
            report = {"revision": "synthetic", "checks": [
                {"name": "unit", "status": "pass", "log": "unit.log"},
                {"name": "broken", "status": "fail", "log": "broken.log"},
                {"name": "redis", "status": "skip", "log": "redis.log"},
                {"name": "browser", "status": "not-run", "reason": "missing framework"},
                {"name": "lock-integrity", "status": "pass", "observed_sha256": {}},
            ]}
            baseline.write_report(Path(temp), report)
            data = json.loads((Path(temp) / "baseline.json").read_text())
            self.assertEqual(data["counts"], {"pass": 2, "fail": 1, "skip": 1, "not-run": 1})

    def test_nonzero_and_timeout_keep_logs(self):
        with tempfile.TemporaryDirectory() as temp:
            run = baseline.Run(Path(temp), "test")
            failed = run.check("failed", ["sh", "-c", "echo synthetic-failure; exit 7"])
            self.assertEqual(failed["exit_code"], 7)
            self.assertIn("synthetic-failure", (Path(temp) / failed["log"]).read_text())
            timeout = run.check("timeout", ["sleep", "10"], timeout=0.01)
            self.assertTrue(timeout["timeout"])
            self.assertEqual(timeout["status"], "fail")

    def test_zero_exit_with_format_findings_is_failure(self):
        with tempfile.TemporaryDirectory() as temp:
            run = baseline.Run(Path(temp), "test")
            finding = run.check("format", ["sh", "-c", "echo synthetic.go"], empty_output=True)
            self.assertEqual(finding["exit_code"], 0)
            self.assertEqual(finding["status"], "fail")
            clean = run.check("clean-format", ["true"], empty_output=True)
            self.assertEqual(clean["status"], "pass")


class IsolationTests(unittest.TestCase):
    def test_no_host_credentials_database_or_network_in_test_container(self):
        command = baseline.container_command("sha256:synthetic", "case", "/tmp/source", "/tmp/state", "backend", ["make", "test"])
        self.assertEqual(command[command.index("--network") + 1], "none")
        self.assertIn("--cap-drop=ALL", command)
        self.assertNotIn("--privileged", command)
        self.assertNotIn("--env-file", command)
        self.assertNotIn("--publish", command)
        self.assertNotIn("docker.sock", " ".join(command))
        env = [command[i + 1] for i, value in enumerate(command) if value == "--env"]
        self.assertTrue(all("=" in value for value in env))
        self.assertFalse(any("SECRET" in value or "DATABASE_URL" in value or "REDIS_ADDR" in value for value in env))
        self.assertIn("GOTOOLCHAIN=local", env)

    def test_backend_json_flag_preserves_default_target(self):
        root = baseline.ROOT / "backend"
        default = subprocess.check_output(["make", "-n", "test"], cwd=root).decode()
        json_command = subprocess.check_output(["make", "-n", "test", "TEST_FLAGS=-json -count=1"], cwd=root).decode()
        self.assertEqual(default.split(), ["go", "test", "./..."])
        self.assertEqual(json_command.split(), ["go", "test", "-json", "-count=1", "./..."])


if __name__ == "__main__":
    unittest.main()
