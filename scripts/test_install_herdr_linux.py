"""Exercise the runtime downloader without contacting GitHub."""
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import hashlib

spec = importlib.util.spec_from_file_location('install_linux', Path(__file__).with_name('install-herdr-linux.py'))
linux = importlib.util.module_from_spec(spec)
if spec.loader and Path(spec.origin).exists():
    spec.loader.exec_module(linux)


class InstallTests(unittest.TestCase):
    def setUp(self):
        if not hasattr(linux, 'install'):
            self.fail('Linux runtime downloader is missing')
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.destination = Path(self.tmp.name) / 'runtime'
        self.data = b'\x7fELFfixture-runtime'
        self.digest = hashlib.sha256(self.data).hexdigest()

    def install(self, data=None, digest=None):
        with patch.object(linux, 'ASSETS', {'amd64': ('herdr-linux-x86_64', digest or self.digest)}), \
                patch.object(linux.urllib.request, 'urlopen', return_value=io.BytesIO(self.data if data is None else data)):
            return linux.install(self.destination, 'amd64')

    def test_checksum_verified_executable(self):
        binary = self.install()
        self.assertEqual(binary, self.destination / 'herdr')
        self.assertEqual(binary.read_bytes(), self.data)
        self.assertEqual(binary.stat().st_mode & 0o777, 0o755)

    def test_checksum_mismatch_leaves_no_destination(self):
        with self.assertRaisesRegex(ValueError, 'SHA-256'):
            self.install(data=b'wrong')
        self.assertFalse(self.destination.exists())

    def test_download_limit_leaves_no_destination(self):
        with patch.object(linux, 'MAX_BINARY', 4), self.assertRaisesRegex(ValueError, 'limit'):
            self.install()
        self.assertFalse(self.destination.exists())

    def test_existing_destination_is_preserved(self):
        self.destination.mkdir()
        sentinel = self.destination / 'keep'
        sentinel.write_text('original')
        with self.assertRaises(FileExistsError):
            self.install()
        self.assertEqual(sentinel.read_text(), 'original')

    def test_unsupported_architecture_does_not_download(self):
        with patch.object(linux.urllib.request, 'urlopen') as download:
            with self.assertRaisesRegex(ValueError, 'architecture'):
                linux.install(self.destination, '386')
            download.assert_not_called()

    def test_failure_cleans_staging_directory(self):
        with self.assertRaises(ValueError):
            self.install(data=b'wrong')
        self.assertEqual(list(Path(self.tmp.name).iterdir()), [])


if __name__ == '__main__':
    unittest.main()
