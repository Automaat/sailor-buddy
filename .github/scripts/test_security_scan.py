import contextlib
import copy
import datetime as dt
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import security_scan as scan


class NpmPolicyTests(unittest.TestCase):
    def setUp(self):
        self.finding = {"severity": "moderate", "isDirect": False,
                        "nodes": ["node_modules/tool/node_modules/example"], "effects": ["tool"],
                        "via": [{"url": "https://github.com/advisories/GHSA-example"}]}
        self.report = {"auditReportVersion": 2, "vulnerabilities": {"example": self.finding},
                       "metadata": {"vulnerabilities": {"info": 0, "low": 0, "moderate": 1,
                                                        "high": 0, "critical": 0, "total": 1}}}
        self.lock = {"packages": {self.finding["nodes"][0]: {"version": "1.0.0"}}}
        self.policy = {"npm_entries": {"example": {
            "scope": {**self.finding, "via": [self.finding["via"][0]["url"]]},
            "versions": {self.finding["nodes"][0]: "1.0.0"}, "advisories": ["GHSA-example"]}},
            "advisories": {"GHSA-example": {"owner": "Automaat", "evidence": "review.md",
                                           "reviewed": "2026-11-14", "review_before": "2026-12-14"}}}

    def gate(self, day=dt.date(2026, 11, 14)):
        with contextlib.redirect_stdout(io.StringIO()):
            return scan.npm_gate(self.report, self.lock, self.policy, day)

    def test_reviewed_tooling_and_clean_reports_pass(self):
        self.assertEqual(self.gate(), 1)
        self.report["vulnerabilities"] = {}
        self.report["metadata"]["vulnerabilities"].update(moderate=0, total=0)
        self.assertEqual(self.gate(), 0)

    def test_new_advisory_runtime_path_parent_or_version_requires_review(self):
        for field, value in [("via", [{"url": "https://github.com/advisories/GHSA-new"}]),
                             ("nodes", ["node_modules/example"]), ("effects", ["runtime"]),
                             ("isDirect", True)]:
            with self.subTest(field=field):
                original = copy.deepcopy(self.finding[field])
                self.finding[field] = value
                with self.assertRaises(ValueError):
                    self.gate()
                self.finding[field] = original
        self.lock["packages"][self.finding["nodes"][0]]["version"] = "1.0.1"
        with self.assertRaises(ValueError):
            self.gate()

    def test_high_critical_and_expired_dispositions_block(self):
        for severity in ("high", "critical"):
            with self.subTest(severity=severity):
                self.finding["severity"] = severity
                self.report["metadata"]["vulnerabilities"].update(moderate=0, high=0, critical=0)
                self.report["metadata"]["vulnerabilities"][severity] = 1
                with self.assertRaisesRegex(ValueError, "Blocking npm finding"):
                    self.gate()
        self.finding["severity"] = "moderate"
        self.report["metadata"]["vulnerabilities"].update(moderate=1, critical=0)
        for day in (dt.date(2026, 11, 13), dt.date(2026, 12, 14)):
            with self.subTest(day=day), self.assertRaisesRegex(ValueError, "expired"):
                self.gate(day)

    def test_registry_errors_and_inconsistent_counts_block(self):
        for bad in ({"error": {"code": "ENOAUDIT"}}, {"auditReportVersion": 3}):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                scan.npm_gate(bad, self.lock, self.policy, dt.date(2026, 11, 14))
        self.report["metadata"]["vulnerabilities"]["total"] = 0
        with self.assertRaises(ValueError):
            self.gate()

    def test_unrelated_live_disposition_cannot_replace_expired_actual_advisory(self):
        self.policy["advisories"]["GHSA-example"]["review_before"] = "2026-11-14"
        self.policy["advisories"]["GHSA-unrelated"] = {
            "owner": "Automaat", "evidence": "other-review.md",
            "reviewed": "2026-11-14", "review_before": "2026-12-14"}
        self.policy["npm_entries"]["example"]["advisories"] = ["GHSA-unrelated"]
        with self.assertRaisesRegex(ValueError, "does not match actual advisories"):
            self.gate()
        self.finding["via"] = ["parent"]
        self.report["vulnerabilities"]["parent"] = {
            "via": [{"url": "https://github.com/advisories/GHSA-example"}]}
        self.assertEqual(scan.npm_advisories(self.report["vulnerabilities"], "example"),
                         {"GHSA-example"})
        self.report["vulnerabilities"]["parent"]["via"] = ["example"]
        with self.assertRaisesRegex(ValueError, "Cyclic"):
            scan.npm_advisories(self.report["vulnerabilities"], "example")

    def test_scanner_failure_retains_raw_report_and_blocks(self):
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.CompletedProcess([], 2, stdout=b'{"error":{"code":"E503"}}')
            with patch.object(scan.subprocess, "run", return_value=result):
                with self.assertRaisesRegex(ValueError, "exit 2"):
                    scan.collect_npm(Path(directory), {})
            self.assertEqual(json.loads((Path(directory) / "npm-audit.json").read_text()),
                             {"error": {"code": "E503"}})


class ImagePolicyTests(unittest.TestCase):
    def setUp(self):
        self.reference = "sha256:" + "a" * 64
        self.report = {"SchemaVersion": 2, "ArtifactType": "container_image",
                       "ArtifactName": self.reference,
                       "Metadata": {"OS": {"Family": "alpine", "EOSL": False}},
                       "Results": [{"Target": "alpine", "Class": "os-pkgs",
                                    "Type": "alpine",
                                    "Packages": [{"Name": "zlib"}], "Vulnerabilities": []}]}

    def gate(self):
        with contextlib.redirect_stdout(io.StringIO()):
            return scan.image_gate(self.report, self.reference)

    def test_clean_and_lower_findings_pass_but_high_critical_block(self):
        self.assertEqual(self.gate(), 0)
        for severity in ("UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL"):
            with self.subTest(severity=severity):
                self.report["Results"][0]["Vulnerabilities"] = [
                    {"Severity": severity, "VulnerabilityID": "CVE-example"}]
                if severity in {"HIGH", "CRITICAL"}:
                    with self.assertRaisesRegex(ValueError, "Blocking image findings"):
                        self.gate()
                else:
                    self.assertEqual(self.gate(), 1)

    def test_empty_inventory_wrong_image_and_eol_block(self):
        for mutation in (lambda r: r.update(Results=[]),
                         lambda r: r.update(ArtifactName="mutable:tag"),
                         lambda r: r["Metadata"]["OS"].update(EOSL=True)):
            report = copy.deepcopy(self.report)
            mutation(self.report)
            with self.assertRaises(ValueError):
                self.gate()
            self.report = report

    def test_mutable_reference_rejected_before_scanner_runs(self):
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, "immutable"):
                scan.collect_image(Path(directory), "app:latest", "docker", {})

    def test_failed_gate_keeps_post_scan_database_and_image_identity(self):
        self.report["Metadata"].update(ImageID=self.reference, ImageConfig={"architecture": "amd64"})
        self.report["Results"][0]["Vulnerabilities"] = [
            {"Severity": "HIGH", "VulnerabilityID": "CVE-example"}]
        version = {"Version": "0.76.2", "VulnerabilityDB": {"UpdatedAt": "2026-11-14"}}
        evidence = {}
        with tempfile.TemporaryDirectory() as directory:
            def run_scan(command, **kwargs):
                self.assertIn("--output", command)
                (Path(directory) / "trivy.json").write_text(json.dumps(self.report))
                return subprocess.CompletedProcess(command, 0)
            def post_scan_version(command, **kwargs):
                self.assertTrue((Path(directory) / "trivy.json").exists())
                return json.dumps(version)
            with patch.object(scan.subprocess, "run", side_effect=run_scan), \
                    patch.object(scan.subprocess, "check_output", side_effect=post_scan_version), \
                    contextlib.redirect_stdout(io.StringIO()):
                with self.assertRaisesRegex(ValueError, "Blocking image findings"):
                    scan.collect_image(Path(directory), self.reference, "docker", evidence)
        self.assertEqual(evidence["trivy"], version)
        self.assertEqual(evidence["image_id"], self.reference)


if __name__ == "__main__":
    unittest.main()
