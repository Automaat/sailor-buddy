import pathlib
import subprocess
import tempfile
import unittest

SCRIPT = pathlib.Path(__file__).with_name("release-version.sh")


class ReleaseVersionTests(unittest.TestCase):
    def test_supported_versions(self):
        cases = [("", "", "v0.1.0"), ("", "v1.9.3", "v1.10.0"),
                 ("v2.0.1", "v1.9.3", "v2.0.1"),
                 ("v2.0.0-rc.1", "", "v2.0.0-rc.1")]
        for requested, previous, expected in cases:
            with self.subTest(requested=requested, previous=previous):
                result = subprocess.run(["bash", str(SCRIPT), requested, previous],
                                        capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.strip(), expected)

    def test_untrusted_values_are_rejected_without_execution(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = pathlib.Path(directory) / "executed"
            values = ["$(touch " + str(marker) + ")", "`touch " + str(marker) + "`",
                      'v1.2.3"; touch ' + str(marker) + '; #',
                      "v1.2.3\nversion=v9.9.9", "--help", "v01.2.3", "not-a-version"]
            for value in values:
                for requested, previous in [(value, ""), ("", value)]:
                    with self.subTest(requested=requested, previous=previous):
                        result = subprocess.run(["bash", str(SCRIPT), requested, previous],
                                                capture_output=True, text=True)
                        self.assertNotEqual(result.returncode, 0)
                        self.assertEqual(result.stdout, "")
                        self.assertFalse(marker.exists())


if __name__ == "__main__":
    unittest.main()
