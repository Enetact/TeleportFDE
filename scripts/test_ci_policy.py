"""Regression coverage for CI cost controls and mandatory main/tag validation."""

import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("ci-policy.py")
SPEC = importlib.util.spec_from_file_location("ci_policy", SCRIPT)
POLICY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(POLICY)


def pr(action="synchronize", labels=(), changed_label=None):
    event = {"action": action, "pull_request": {"labels": [{"name": label} for label in labels]}}
    if changed_label is not None:
        event["label"] = {"name": changed_label}
    return event


class PolicyTests(unittest.TestCase):
    def test_event_matrix(self):
        cases = [
            ("feature push", "push", "refs/heads/feature/example", {}, (True, False)),
            ("arbitrary branch push", "push", "refs/heads/docs", {}, (True, False)),
            ("develop push", "push", "refs/heads/develop", {}, (True, False)),
            ("main push", "push", "refs/heads/main", {}, (True, True)),
            ("merge push to main", "push", "refs/heads/main", {"head_commit": {"message": "Merge pull request #4"}}, (True, True)),
            ("version tag", "push", "refs/tags/v1.0.0", {}, (True, True)),
            ("deleted version tag", "push", "refs/tags/v1.0.0", {"deleted": True}, (False, False)),
            ("new PR off", "pull_request", "refs/pull/4/merge", pr("opened"), (False, False)),
            ("PR update off", "pull_request", "refs/pull/4/merge", pr(), (False, False)),
            ("enable label", "pull_request", "refs/pull/4/merge", pr("labeled", ["ci:full"], "ci:full"), (True, True)),
            ("labeled PR update", "pull_request", "refs/pull/4/merge", pr(labels=["ci:full"]), (True, True)),
            ("labeled PR reopen", "pull_request", "refs/pull/4/merge", pr("reopened", ["ci:full"]), (True, True)),
            ("remove enable label", "pull_request", "refs/pull/4/merge", pr("unlabeled", [], "ci:full"), (False, False)),
            ("unrelated label added", "pull_request", "refs/pull/4/merge", pr("labeled", ["ci:full", "docs"], "docs"), (False, False)),
            ("unrelated label removed", "pull_request", "refs/pull/4/merge", pr("unlabeled", ["ci:full"], "docs"), (False, False)),
            ("closed PR does not duplicate merge push", "pull_request", "refs/heads/main", pr("closed", ["ci:full"]), (False, False)),
            ("manual default", "workflow_dispatch", "refs/heads/feature/example", {}, (True, False)),
            ("manual true string", "workflow_dispatch", "refs/heads/feature/example", {"inputs": {"cluster_tests": "true"}}, (True, True)),
            ("manual true bool", "workflow_dispatch", "refs/heads/feature/example", {"inputs": {"cluster_tests": True}}, (True, True)),
            ("manual false string", "workflow_dispatch", "refs/heads/feature/example", {"inputs": {"cluster_tests": "false"}}, (True, False)),
            ("manual false bool", "workflow_dispatch", "refs/heads/feature/example", {"inputs": {"cluster_tests": False}}, (True, False)),
            ("main cannot opt out", "workflow_dispatch", "refs/heads/main", {"inputs": {"cluster_tests": False}}, (True, True)),
        ]
        for name, event_name, ref, event, expected in cases:
            with self.subTest(name=name):
                self.assertEqual(POLICY.select_checks(event_name, ref, event)[:2], expected)

    def test_unsupported_event_fails(self):
        with self.assertRaises(ValueError):
            POLICY.select_checks("pull_request_target", "refs/heads/main", {})

    def test_github_output_and_summary(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            event = root / "event.json"
            output = root / "output"
            summary = root / "summary"
            event.write_text(json.dumps(pr("labeled", ["ci:full"], "ci:full")), encoding="utf-8")
            output.write_text("existing=value\n", encoding="utf-8")
            env = {**os.environ, "GITHUB_EVENT_NAME": "pull_request", "GITHUB_REF": "refs/pull/4/merge",
                   "GITHUB_EVENT_PATH": str(event), "GITHUB_OUTPUT": str(output),
                   "GITHUB_STEP_SUMMARY": str(summary)}
            subprocess.run([sys.executable, str(SCRIPT)], env=env, check=True, capture_output=True)
            self.assertEqual(output.read_text(encoding="utf-8"), "existing=value\nquality=true\nfull=true\n")
            self.assertIn("Full PR checks enabled", summary.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
