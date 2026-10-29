"""Product name and release-path contracts for the Hergo rebrand."""
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent.parent


class BrandingTests(unittest.TestCase):
    def test_name_and_stable_update_identity(self):
        config = json.loads((ROOT / 'mygo.json').read_text())
        self.assertEqual(config['name'], 'Hergo')
        self.assertEqual(config['identifier'], 'dev.herdr.ui')
        self.assertEqual(config['updates']['github'], 'Maidang1/herdr-ui')
        self.assertEqual(config['updates']['publicKey'], 'wxS8IPnKAsClT2jm8fQBLI2AkiVFZFSOLC64H+OIZSw=')
        self.assertEqual(config['linux']['command'], 'herdr-ui')

    def test_visible_branding(self):
        for name in ('cmd/herdr-ui/main.go', 'cmd/herdr-ui/state.go',
                     'internal/ui/settings.go', 'internal/ui/settings_updates.go'):
            source = (ROOT / name).read_text()
            self.assertNotIn('Herdr UI', source)
            self.assertIn('Hergo', source)
        self.assertTrue((ROOT / 'README.md').read_text().startswith('# Hergo\n'))

    def test_release_uses_renamed_packager_outputs(self):
        workflow = (ROOT / '.github/workflows/release.yml').read_text()
        self.assertIn('dist/darwin-universal/Hergo ${{ steps.version.outputs.version }}.dmg', workflow)
        self.assertIn('hergo-$version-linux-${{ matrix.arch }}.tar.gz', workflow)
        self.assertIn('hergo_${version}_${{ matrix.arch }}.deb', workflow)
        self.assertIn('"$dir/hergo" -local-fixture', workflow)
        self.assertNotIn('Herdr UI', workflow)


if __name__ == '__main__':
    unittest.main()
