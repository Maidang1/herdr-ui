#!/usr/bin/env python3
"""Fetch the pinned official Herdr Windows x64 runtime into an explicit directory."""
from __future__ import annotations

import argparse
import hashlib
import os
from pathlib import Path, PurePosixPath
import shutil
import stat
import tempfile
import urllib.request
import zipfile

VERSION = "0.9.5"
SHA256 = "d2c6c9c971c43a0bfdde8c692a9a1a7a45c3e4807cd01ae45662b4734fac1cbf"
URL = f"https://github.com/herdrdev/herdr/releases/download/v{VERSION}/herdr-windows-x86_64.zip"
MAX_ARCHIVE = 256 * 1024 * 1024
MAX_EXTRACTED = 768 * 1024 * 1024


def install(destination: Path) -> Path:
    destination = destination.resolve()
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="herdr-download-", dir=destination.parent) as tmp:
        archive = Path(tmp) / "runtime.zip"
        digest = hashlib.sha256()
        received = 0
        request = urllib.request.Request(URL, headers={"User-Agent": "herdr-ui-windows-ci"})
        with urllib.request.urlopen(request, timeout=60) as response, archive.open("wb") as output:
            while chunk := response.read(1024 * 1024):
                received += len(chunk)
                if received > MAX_ARCHIVE:
                    raise ValueError("Herdr archive exceeds download limit")
                digest.update(chunk)
                output.write(chunk)
        if digest.hexdigest() != SHA256:
            raise ValueError("Herdr archive SHA-256 mismatch")
        stage = Path(tmp) / "unpacked"
        stage.mkdir()
        with zipfile.ZipFile(archive) as zipped:
            members = zipped.infolist()
            if len(members) > 4096 or sum(member.file_size for member in members) > MAX_EXTRACTED:
                raise ValueError("Herdr archive exceeds extraction limits")
            names: set[str] = set()
            for member in members:
                # Reject Windows separators, drive/ADS names, traversal and symlinks.
                name = member.filename
                path = PurePosixPath(name)
                mode = member.external_attr >> 16
                if (not name or "\\" in name or ":" in name or path.is_absolute()
                        or ".." in path.parts or stat.S_ISLNK(mode)
                        or any(part.endswith((".", " ")) or part.split(".", 1)[0].upper() in {"CON", "PRN", "AUX", "NUL", *(f"COM{i}" for i in range(1, 10)), *(f"LPT{i}" for i in range(1, 10))} for part in path.parts)):
                    raise ValueError(f"Unsafe Herdr archive path: {name!r}")
                normalized = str(path).casefold().rstrip("/")
                if normalized in names:
                    raise ValueError(f"Duplicate Herdr archive path: {name!r}")
                names.add(normalized)
                target = stage.joinpath(*path.parts)
                if member.is_dir():
                    target.mkdir(parents=True, exist_ok=True)
                else:
                    target.parent.mkdir(parents=True, exist_ok=True)
                    with zipped.open(member) as source, target.open("xb") as output:
                        shutil.copyfileobj(source, output, 1024 * 1024)
        executables = list(stage.rglob("herdr.exe"))
        if len(executables) != 1:
            raise ValueError("Expected exactly one herdr.exe in official archive")
        relative = executables[0].relative_to(stage)
        if destination.exists():
            raise FileExistsError(f"Destination already exists: {destination}")
        os.replace(stage, destination)
        return destination / relative


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--destination", type=Path, required=True)
    args = parser.parse_args()
    executable = install(args.destination)
    print(executable)
    if github_env := os.environ.get("GITHUB_ENV"):
        with open(github_env, "a", encoding="utf-8") as output:
            output.write(f"HERDR_TEST_BINARY={executable}\n")


if __name__ == "__main__":
    main()
