"""Contracts for Linux packaging and release wiring (no network or GUI)."""
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent.parent


class LinuxReleaseTests(unittest.TestCase):
    def test_native_only_package(self):
        linux = json.loads((ROOT / 'mygo.json').read_text()).get('linux', {})
        self.assertIs(linux.get('webview'), False)
        self.assertEqual(linux.get('command'), 'herdr-ui')
        self.assertIn('Development', linux.get('categories', []))

    def test_both_native_architectures_gate_publication(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertIn('needs: [macos, windows, linux]', workflow)
        self.assertIn('runner: ubuntu-22.04', workflow)
        self.assertIn('runner: ubuntu-22.04-arm', workflow)
        self.assertIn('name: release-linux-${{ matrix.arch }}', workflow)
        self.assertIn('xvfb-run -a', workflow)
        self.assertIn('cmp release-assets/install-linux-amd64.sh release-assets/install-linux-arm64.sh', workflow)

    def test_unix_integration_available_on_linux(self):
        for name in ('m0', 'm1', 'm2', 'm3', 'keyboard'):
            source = (ROOT / 'integration' / f'{name}_test.go').read_text()
            self.assertEqual(source.splitlines()[0], '//go:build integration && (darwin || linux)')
            self.assertIn('unixTestBinary(t)', source)


if __name__ == '__main__':
    unittest.main()
