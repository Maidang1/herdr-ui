#!/usr/bin/env python3
"""Fetch checksum-pinned Herdr Linux into a new explicit test directory."""
from __future__ import annotations

import argparse
import hashlib
import os
from pathlib import Path
import tempfile
import urllib.request

VERSION = '0.9.5'
ASSETS = {
    'amd64': ('herdr-linux-x86_64', '14d9e8bc8b4a9e0118d77c32266f76d461f09b40342eb23752960bd89f59929d'),
    'arm64': ('herdr-linux-aarch64', '17d7536f2da50859523680ef279dffcac8e4a235f4e46fe8da6c2ec44141897c'),
}
MAX_BINARY = 256 * 1024 * 1024


def install(destination: Path, arch: str) -> Path:
    if arch not in ASSETS:
        raise ValueError(f'Unsupported Linux architecture: {arch}')
    destination = destination.absolute()
    if destination.exists() or destination.is_symlink():
        raise FileExistsError(f'Destination already exists: {destination}')
    destination.parent.mkdir(parents=True, exist_ok=True)
    name, expected = ASSETS[arch]
    url = f'https://github.com/herdrdev/herdr/releases/download/v{VERSION}/{name}'
    with tempfile.TemporaryDirectory(prefix='herdr-download-', dir=destination.parent) as tmp:
        stage = Path(tmp) / 'runtime'
        stage.mkdir()
        binary = stage / 'herdr'
        digest = hashlib.sha256()
        received = 0
        request = urllib.request.Request(url, headers={'User-Agent': 'herdr-ui-linux-ci'})
        with urllib.request.urlopen(request, timeout=60) as response, binary.open('xb') as output:
            while chunk := response.read(1024 * 1024):
                received += len(chunk)
                if received > MAX_BINARY:
                    raise ValueError('Herdr binary exceeds download limit')
                digest.update(chunk)
                output.write(chunk)
        if digest.hexdigest() != expected:
            raise ValueError('Herdr binary SHA-256 mismatch')
        binary.chmod(0o755)
        # Reserve the destination atomically; never replace another directory,
        # even if it appeared during the download. Publish only verified bytes.
        destination.mkdir()
        try:
            os.replace(binary, destination / 'herdr')
        except OSError:
            destination.rmdir()
            raise
    return destination / 'herdr'


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--destination', type=Path, required=True)
    parser.add_argument('--arch', choices=ASSETS, required=True)
    args = parser.parse_args()
    executable = install(args.destination, args.arch)
    print(executable)
    if github_env := os.environ.get('GITHUB_ENV'):
        with open(github_env, 'a', encoding='utf-8') as output:
            output.write(f'HERDR_TEST_BINARY={executable}\n')


if __name__ == '__main__':
    main()
