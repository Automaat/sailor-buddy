"""Collect complete scanner reports and apply the policy in docs/security-scanning.md."""

import argparse
import datetime as dt
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
POLICY = ROOT / ".github/security-dispositions.json"
SEVERITIES = {"info", "low", "moderate", "high", "critical"}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def npm_advisories(findings, name, seen=frozenset()):
    require(name not in seen, f"Cyclic npm advisory graph: {name}")
    ids = set()
    for via in findings[name]["via"]:
        if isinstance(via, str):
            ids.update(npm_advisories(findings, via, seen | {name}))
        else:
            require(via["url"].startswith("https://github.com/advisories/GHSA-"),
                    "Unsupported npm advisory identity")
            ids.add(via["url"].rsplit("/", 1)[-1])
    require(ids, f"Missing npm advisory identity: {name}")
    return ids


def npm_gate(report, lock, policy, today):
    require(report.get("auditReportVersion") == 2 and "error" not in report,
            "Incomplete or unsupported npm report")
    findings = report["vulnerabilities"]
    counts = report["metadata"]["vulnerabilities"]
    require(isinstance(findings, dict) and counts["total"] == len(findings),
            "npm finding count mismatch")
    for severity in SEVERITIES:
        require(counts[severity] == sum(v["severity"] == severity for v in findings.values()),
                "npm severity count mismatch")
    for name, finding in findings.items():
        require(finding["severity"] in {"info", "low", "moderate"},
                f"Blocking npm finding: {name} ({finding['severity']})")
        allowed = policy["npm_entries"].get(name)
        require(allowed is not None, f"Unreviewed npm finding: {name}")
        actual = {key: finding[key] for key in ("severity", "isDirect", "nodes", "effects")}
        actual["via"] = sorted(v if isinstance(v, str) else v["url"] for v in finding["via"])
        for key in ("nodes", "effects"):
            actual[key] = sorted(actual[key])
        require(actual == allowed["scope"], f"Changed npm exposure: {name}")
        require(set(allowed["advisories"]) == npm_advisories(findings, name),
                f"Disposition does not match actual advisories: {name}")
        require(set(allowed["versions"]) == set(finding["nodes"]),
                f"Incomplete reviewed scope: {name}")
        require(all(lock["packages"][node]["version"] == version
                    for node, version in allowed["versions"].items()),
                f"Changed reviewed dependency version: {name}")
        for advisory in allowed["advisories"]:
            disposition = policy["advisories"][advisory]
            reviewed = dt.date.fromisoformat(disposition["reviewed"])
            expires = dt.date.fromisoformat(disposition["review_before"])
            require(disposition["owner"] and disposition["evidence"] and
                    reviewed <= today < expires and (expires - reviewed).days <= 31,
                    f"Missing or expired disposition: {advisory}")
        print(f"Reviewed build-tool finding: {name} ({finding['severity']})")
    return len(findings)


def image_gate(report, reference):
    require(report.get("SchemaVersion") == 2 and report.get("ArtifactType") == "container_image",
            "Incomplete or unsupported Trivy report")
    require(report["ArtifactName"] == reference, "Trivy scanned a different image")
    require(report["Metadata"].get("OS", {}).get("Family") == "alpine",
            "Missing expected Alpine OS inventory")
    results = report["Results"]
    require(any(r.get("Class") == "os-pkgs" and r.get("Type") == "alpine" and r.get("Packages")
                for r in results),
            "Missing OS package inventory")
    require(not report["Metadata"]["OS"].get("EOSL", False), "Unsupported image OS")
    total = 0
    blocked = []
    for result in results:
        for finding in result.get("Vulnerabilities", []):
            severity = finding["Severity"]
            require(severity in {"UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL"},
                    "Unknown Trivy severity")
            total += 1
            label = f"{result['Target']}: {finding['VulnerabilityID']} ({severity})"
            print(label)
            if severity in {"HIGH", "CRITICAL"}:
                blocked.append(label)
    require(not blocked, "Blocking image findings: " + "; ".join(blocked))
    return total


def collect_npm(directory, metadata):
    result = subprocess.run(["npm", "audit", "--json", "--include=dev", "--include=optional",
                             "--include=peer", "--audit-level=low"], cwd=ROOT / "frontend",
                            stdout=subprocess.PIPE, check=False)
    path = directory / "npm-audit.json"
    path.write_bytes(result.stdout)
    require(result.returncode in (0, 1), f"npm audit failed: exit {result.returncode}")
    report = json.loads(result.stdout)
    lock_path = ROOT / "frontend/package-lock.json"
    metadata.update(lockfile_sha256=hashlib.sha256(lock_path.read_bytes()).hexdigest(),
                    scanner_version=subprocess.check_output(["npm", "--version"], text=True).strip())
    count = npm_gate(report, json.loads(lock_path.read_text()), json.loads(POLICY.read_text()),
                     dt.datetime.now(dt.timezone.utc).date())
    require(result.returncode == (1 if count else 0), "npm exit code disagrees with report")
    metadata["finding_entries"] = count


def collect_image(directory, reference, source, metadata):
    require(re.fullmatch(r"(?:[a-z0-9./:_-]+@)?sha256:[0-9a-f]{64}", reference),
            "Image reference must contain an immutable SHA-256 digest")
    path = directory / "trivy.json"
    subprocess.run(["trivy", "image", "--image-src", source, "--scanners", "vuln",
                    "--pkg-types", "os,library", "--severity", "UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL",
                    "--ignore-unfixed=false", "--ignorefile", "", "--exit-code", "0",
                    "--list-all-pkgs", "--no-progress", "--timeout", "10m",
                    "--format", "json", "--output", str(path), reference], check=True)
    report = json.loads(path.read_text())
    version = json.loads(subprocess.check_output(["trivy", "--version", "--format", "json"], text=True))
    metadata.update(image_reference=reference, image_id=report["Metadata"]["ImageID"],
                    repo_digests=report["Metadata"].get("RepoDigests", []),
                    platform=report["Metadata"]["ImageConfig"]["architecture"], trivy=version)
    metadata["finding_entries"] = image_gate(report, reference)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=("npm", "image"))
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--image")
    parser.add_argument("--source", choices=("docker", "remote"), default="docker")
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    metadata = {"source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT,
                                                         text=True).strip(),
                "collected_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                "kind": args.kind, "policy_passed": False}
    if args.kind == "image":
        metadata["image_reference"] = args.image
    try:
        if args.kind == "npm":
            collect_npm(args.output, metadata)
        else:
            collect_image(args.output, args.image or "", args.source, metadata)
        metadata["policy_passed"] = True
        print("Security scan policy passed")
        return 0
    except (ValueError, KeyError, TypeError, OSError, subprocess.SubprocessError) as error:
        metadata["error"] = str(error)
        print(f"Security scan failed: {error}", file=sys.stderr)
        return 1
    finally:
        (args.output / "metadata.json").write_text(json.dumps(metadata, indent=2) + "\n")


if __name__ == "__main__":
    sys.exit(main())
