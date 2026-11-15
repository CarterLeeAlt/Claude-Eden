import gzip
import base64
import hashlib
import io
from pathlib import Path
import runpy
import tarfile
import unittest
from unittest.mock import patch
import zipfile

audit = runpy.run_path(Path(__file__).resolve().parents[1] / "scripts/check-guest-runtime.py")


class GuestRuntimeAuditTests(unittest.TestCase):
    def test_detects_names_case_and_both_utf16_alignments(self):
        for data in (b"CAGE_PATH", "Cage.Guest".encode("utf-16-le"),
                     b"X" + "Cage.Guest".encode("utf-16-be")):
            with self.subTest(data=data):
                self.assertTrue(audit["inspect_blob"]("component.bin", data))
        self.assertTrue(audit["inspect_blob"]("cage-helper", b""))

    def test_host_brand_is_distinct_from_credentials(self):
        self.assertFalse(audit["inspect_blob"]("credentials.json", b"credentials developer browser workbench"))
        for data in (b"Claude Eden", b"EDEN_CONFIG", b"dev.eden.Guest", b"Eden.Guest"):
            with self.subTest(data=data):
                self.assertTrue(audit["inspect_blob"]("component", data))

    def test_checks_nested_archives_and_link_destinations(self):
        tar = io.BytesIO()
        with tarfile.open(fileobj=tar, mode="w") as archive:
            entry = tarfile.TarInfo("open")
            entry.type = tarfile.SYMTYPE
            entry.linkname = "/usr/local/lib/cage-open"
            archive.addfile(entry)
        zipped = io.BytesIO()
        with zipfile.ZipFile(zipped, "w", compression=zipfile.ZIP_DEFLATED) as archive:
            archive.writestr("runtime.tar.gz", gzip.compress(tar.getvalue()))
        result = audit["inspect_blob"]("runtime.zip", zipped.getvalue())
        self.assertTrue(any("!open" in item["file"] for item in result))

    def test_does_not_disclose_neighboring_credentials(self):
        result = audit["inspect_blob"]("runtime", b"secret-token=never-print-this CAGE_TOKEN")
        self.assertTrue(result)
        self.assertNotIn("never-print-this", str(result))

    def test_corrupt_archive_is_an_error(self):
        with self.assertRaises((EOFError, OSError)):
            audit["inspect_blob"]("broken.gz", b"\x1f\x8bnot-a-valid-archive")

    def test_incidental_api_names_do_not_hide_adjacent_product_identifiers(self):
        result = audit["inspect_blob"]("component.dll", b"DsReplicaGetInfoW Cage.Guest")
        self.assertEqual(sum("incidental" in item for item in result), 1)
        self.assertEqual(sum("incidental" not in item for item in result), 1)
        wide = audit["inspect_blob"]("component.dll", ("\U0001f680DsReplicaGetInfoW").encode("utf-16-le"))
        self.assertTrue(wide)
        self.assertTrue(all("incidental" in item for item in wide))
        self.assertTrue(any("incidental" not in item for item in audit["inspect_blob"]("Cage.exe", b"DsReplicaGetInfoW")))

    def test_only_pinned_complete_certificates_are_classified_as_incidental(self):
        pem = b"-----BEGIN CERTIFICATE-----\ncage\n-----END CERTIFICATE-----"
        digest = hashlib.sha256(base64.b64decode(b"cage")).hexdigest()
        with patch.dict(audit["ROOT_CERTIFICATES"], {digest: "fixture"}):
            self.assertTrue(all("incidental" in item for item in audit["inspect_blob"]("roots", pem)))
            changed = pem.replace(b"cage", b"cAgE")
            self.assertTrue(any("incidental" not in item for item in audit["inspect_blob"]("roots", changed)))


if __name__ == "__main__":
    unittest.main()
