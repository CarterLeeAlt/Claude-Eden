"""Check delivered guest sources and artifacts for product identifiers.

Names, script contents, UTF-16 metadata and nested zip/tar/gzip resources are
checked without extracting archives. Run --sources for the embedded inputs;
pass installed guest payloads (or captured guest inventories) as paths to check
build and runtime output. Host-only installers are not guest payloads.
"""
import argparse
import base64
import binascii
import gzip
import hashlib
import io
import json
from pathlib import Path
import re
import shlex
import sys
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
PATTERN = r"cage|claude[-_ ]eden|\beden[_-]|dev\.eden|Eden\.Guest"
TEXT = re.compile(PATTERN, re.IGNORECASE)
BYTES = re.compile(PATTERN.encode(), re.IGNORECASE)
MAX_RESOURCE = 512 * 1024 * 1024
# Public runtime API names contain the incidental substring "caGe". Altering
# these Microsoft identifiers would break enum/native API contracts.
RUNTIME_IDENTIFIERS = (
    "NorthAmericaGermanStandardFanfold", "NorthAmericaGermanLegalFanfold",
    "DsReplicaGetInfo2W", "DsReplicaGetInfoW",
)
# Tailscale embeds these unmodified public roots. Verify the complete DER hash,
# not a filename or a generic Base64 pattern, before classifying an occurrence.
ROOT_CERTIFICATES = {
    "96bcec06264976f37460779acf28c5a7cfe8a3c0aae11a8ffcee05c0bddf08c6":
        "https://letsencrypt.org/certs/isrgrootx1.pem",
    "e57b7e6f150c419102e8d5c055729ff967b9d1a829bf00cec89ca604ebf4a86f":
        "https://letsencrypt.org/certs/gen-y/root-yr.pem",
}


def incidental_spans(data):
    spans = []
    for identifier in RUNTIME_IDENTIFIERS:
        for encoding in ("ascii", "utf-16-le", "utf-16-be"):
            value = identifier.encode(encoding)
            for match in re.finditer(re.escape(value), data):
                spans.append((*match.span(), "Microsoft runtime identifier: " + identifier))
    for match in re.finditer(rb"-----BEGIN CERTIFICATE-----\r?\n([A-Za-z0-9+/=\r\n]+)-----END CERTIFICATE-----", data):
        try:
            der = base64.b64decode(re.sub(rb"\s", b"", match.group(1)), validate=True)
        except binascii.Error:
            continue
        digest = hashlib.sha256(der).hexdigest()
        if digest in ROOT_CERTIFICATES:
            spans.append((*match.span(), "Public root certificate SHA256=" + digest + " " + ROOT_CERTIFICATES[digest]))
    return spans


def findings(name, data):
    matches = []
    spans = incidental_spans(data)

    def content_match(kind, start, end, identifier):
        item = {"file": name, "kind": kind, "offset": start, "identifier": identifier}
        for begin, finish, reason in spans:
            if begin <= start and end <= finish:
                item["incidental"] = reason
                break
        matches.append(item)

    for match in TEXT.finditer(name):
        matches.append({"file": name, "kind": "name", "identifier": match.group()})
    for match in BYTES.finditer(data):
        content_match("bytes", match.start(), match.end(), match.group().decode("ascii"))
    # PE/.NET metadata and Windows configuration can contain UTF-16 at either
    # byte alignment. Only matching tokens are reported, never adjacent data.
    for encoding in ("utf-16-le", "utf-16-be"):
        for alignment in (0, 1):
            text = data[alignment:].decode(encoding, errors="replace")
            for match in TEXT.finditer(text):
                start = alignment + len(text[:match.start()].encode(encoding))
                content_match(encoding, start, start + len(match.group().encode(encoding)), match.group())
    return matches


def bounded_read(stream):
    data = stream.read(MAX_RESOURCE + 1)
    if len(data) > MAX_RESOURCE:
        raise ValueError("guest resource exceeds the 512 MiB inspection limit")
    return data


def inspect_blob(name, data, depth=0):
    if depth > 8:
        raise ValueError("guest archive nesting exceeds eight levels")
    result = findings(name, data)
    lower = name.lower()
    stream = io.BytesIO(data)
    if zipfile.is_zipfile(io.BytesIO(data)):
        with zipfile.ZipFile(stream) as archive:
            for item in archive.infolist():
                if item.is_dir():
                    result.extend(findings(name + "!" + item.filename, b""))
                else:
                    with archive.open(item) as content:
                        result.extend(inspect_blob(name + "!" + item.filename, bounded_read(content), depth + 1))
    elif data.startswith(b"\x1f\x8b"):
        with gzip.GzipFile(fileobj=stream) as archive:
            result.extend(inspect_blob(name + "!gzip", bounded_read(archive), depth + 1))
    elif lower.endswith((".tar", ".tar!gzip", ".tar.gz!gzip", ".tgz!gzip")) or len(data) >= 262 and data[257:262] == b"ustar":
        with tarfile.open(fileobj=stream, mode="r:") as archive:
            for item in archive:
                target = name + "!" + item.name
                if item.issym() or item.islnk():
                    result.extend(findings(target, item.linkname.encode()))
                elif item.isfile():
                    with archive.extractfile(item) as content:
                        result.extend(inspect_blob(target, bounded_read(content), depth + 1))
                else:
                    result.extend(findings(target, b""))
    return result


def guest_sources():
    runtime = list((ROOT / "internal").glob("*/guest-setup.sh"))
    if len(runtime) != 1:
        raise ValueError("expected exactly one guest provisioning source directory")
    paths = set()
    for directory in (runtime[0].parent, ROOT / "internal/workbench"):
        for source in directory.glob("*.go"):
            if source.name.endswith("_test.go"):
                continue
            for declaration in re.findall(r"^//go:embed (.+)$", source.read_text(encoding="utf-8"), re.MULTILINE):
                for pattern in shlex.split(declaration):
                    if pattern.startswith("web/"):
                        continue
                    found = list(directory.glob(pattern))
                    if not found:
                        raise ValueError(f"missing embedded guest input: {source.name}: {pattern}")
                    for path in found:
                        if path.is_dir():
                            paths.update(p for p in path.rglob("*") if p.is_file())
                        else:
                            paths.add(path)
    paths.update(p for p in (ROOT / "tools/tailnet").glob("*.go") if not p.name.endswith("_test.go"))
    paths.add(ROOT / "tools/tailnet/go.mod")
    return sorted(paths)


def inspect_path(path):
    if path.is_symlink():
        return findings(path.name, str(path.readlink()).encode())
    with path.open("rb") as content:
        return inspect_blob(path.name, bounded_read(content))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sources", action="store_true")
    parser.add_argument("paths", type=Path, nargs="*")
    args = parser.parse_args()
    if not args.sources and not args.paths:
        parser.error("select --sources or supply guest artifact paths")
    paths = set(guest_sources()) if args.sources else set()
    for path in args.paths:
        if not path.exists() and not path.is_symlink():
            raise FileNotFoundError(path)
        if path.is_dir():
            paths.update(path.rglob("*"))
        else:
            paths.add(path)
    result, count = [], 0
    for path in sorted(paths):
        if path.is_dir() and not path.is_symlink():
            result.extend(findings(path.name, b""))
            continue
        count += 1
        for item in inspect_path(path):
            item["file"] = str(path) + item["file"][len(path.name):]
            result.append(item)
    unresolved = [item for item in result if "incidental" not in item]
    incidental = [item for item in result if "incidental" in item]
    print(json.dumps({"files": count, "matches": unresolved, "incidental_matches": incidental}, ensure_ascii=False, indent=2))
    return 1 if unresolved else 0


if __name__ == "__main__":
    sys.exit(main())
