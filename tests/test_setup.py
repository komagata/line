#!/usr/bin/env python3
"""Source setup tests; all commands and installation destinations are temporary."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class SetupTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='line-setup-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / 'source with spaces'
        (self.source / 'scripts').mkdir(parents=True)
        (self.source / 'backend').mkdir()
        for name in ('setup', 'setup.py', 'build', 'launcher'):
            if (ROOT / 'scripts' / name).exists():
                shutil.copy2(ROOT / 'scripts' / name, self.source / 'scripts' / name)
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.log = self.root / 'calls'
        self.env = dict(os.environ, HOME=str(self.root / 'home'), PATH=str(self.bin),
                        XDG_DATA_HOME=str(self.root / 'data with spaces'),
                        CALLS=str(self.log), GOCACHE=str(self.root / 'gocache'),
                        FAIL='', CATALOG='[]', PLUGINS='[{"id":"io.github.komagata.line"}]')
        self.destination = self.root / 'home/.config/omarchy/plugins/io.github.komagata.line'
        for tool in ('bash', 'dirname', 'python3'):
            (self.bin / tool).symlink_to(sys.executable if tool == 'python3' else shutil.which(tool))
        for tool in ('go', 'git', 'secret-tool', 'omarchy', 'omarchy-shell', 'omarchy-plugin-catalog'):
            self.command(self.bin / tool, '''import json, os, sys
from pathlib import Path
name = Path(sys.argv[0]).name
with open(os.environ['CALLS'], 'a') as log: log.write(json.dumps([name, *sys.argv[1:]]) + '\\n')
if name == 'go' and sys.argv[1:] == ['env', 'GOVERSION']: print(os.environ.get('GO_VERSION', 'go1.27.0'))
elif name == 'omarchy-plugin-catalog': print(os.environ['CATALOG'])
elif name == 'omarchy' and sys.argv[1:] == ['plugin', 'list', '--json']: print(os.environ['PLUGINS'])
if (os.environ['FAIL'] == 'download' and name == 'go' and sys.argv[1:] == ['mod', 'download']) or os.environ['FAIL'] == name or (os.environ['FAIL'] == 'enable' and sys.argv[1:3] == ['plugin', 'enable']): sys.exit(7)
''')
        self.command(self.source / 'scripts/package.py', '''import json, os, sys
from pathlib import Path
with open(os.environ['CALLS'], 'a') as log: log.write(json.dumps(['build', os.environ.get('GO_BIN')]) + '\\n')
if os.environ['FAIL'] == 'build': sys.exit(8)
''')
        self.command(self.source / 'scripts/install', '''import json, os, sys
from pathlib import Path
with open(os.environ['CALLS'], 'a') as log: log.write(json.dumps(['install', *sys.argv[1:]]) + '\\n')
if os.environ['FAIL'] == 'install': sys.exit(9)
p = Path(sys.argv[1]); p.mkdir(parents=True); (p / 'sentinel').write_text('installed')
''')

    def command(self, path, body):
        path.write_text('#!' + sys.executable + '\n' + body)
        path.chmod(0o755)

    def run_setup(self):
        if not (self.source / 'scripts/setup').exists():
            self.fail('scripts/setup must exist for the README one-line install')
        return subprocess.run([str(self.source / 'scripts/setup')], env=self.env,
                              text=True, capture_output=True)

    def launcher(self):
        return self.root / 'data with spaces/applications/io.github.komagata.line.desktop'

    def calls(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_success_and_repeat_preserve_install(self):
        result = self.run_setup()
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        names = [call[0] for call in calls]
        self.assertLess(names.index('go', 1), names.index('build'))
        self.assertIn(['go', 'mod', 'download'], calls)
        self.assertIn(['omarchy-shell', 'shell', 'rescanPlugins'], calls)
        self.assertIn(['omarchy', 'plugin', 'enable', 'io.github.komagata.line'], calls)
        self.assertEqual(next(c[1] for c in calls if c[0] == 'build'), str(self.bin / 'go'))
        self.assertTrue(self.launcher().is_file())
        self.assertIn('Exec=omarchy-shell shell summon io.github.komagata.line',
                      self.launcher().read_text())
        self.assertIn('app launcher', result.stdout)
        self.log.unlink()
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertEqual(self.calls(), [])
        self.assertEqual((self.destination / 'sentinel').read_text(), 'installed')

    def test_readme_one_line_with_fixture_clone_and_real_installer(self):
        # The clone boundary is mocked; no network or repository commits occur.
        for name in ('package.py', 'install', 'validate_manifest.py'):
            shutil.copy2(ROOT / 'scripts' / name, self.source / 'scripts' / name)
        for name in ('BarWidget.qml', 'Panel.qml', 'Service.qml', 'manifest.json', 'LICENSE'):
            shutil.copy2(ROOT / name, self.source / name)
        shutil.copytree(ROOT / 'components', self.source / 'components')
        self.command(self.source / 'scripts/build', '''import shutil
from pathlib import Path
root = Path(__file__).resolve().parents[1]
payload = root / 'build/plugin'; (payload / 'bin').mkdir(parents=True)
for name in ('BarWidget.qml', 'Panel.qml', 'Service.qml', 'manifest.json', 'LICENSE'):
    shutil.copy2(root / name, payload / name)
shutil.copytree(root / 'components', payload / 'components')
(payload / 'NOTICE').write_text('Fictional test payload')
(payload / 'bin/line-gui').write_bytes(bytes([127, 69, 76, 70]))
(payload / 'bin/line-gui').chmod(0o755)
''')
        self.command(self.bin / 'git', '''import os, shutil, sys
assert sys.argv[1:] == ['clone', 'https://github.com/komagata/line.git']
shutil.copytree(os.environ['FIXTURE_SOURCE'], 'line')
''')
        self.env['FIXTURE_SOURCE'] = str(self.source)
        checkout = self.root / 'checkout'; checkout.mkdir()
        one_line = next(line for line in (ROOT / 'README.md').read_text().splitlines()
                        if line.startswith('git clone '))
        result = subprocess.run(['/bin/bash', '-c', one_line], cwd=checkout,
                                env=self.env, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.destination / 'bin/line-gui').read_bytes(), b'\x7fELF')
        self.assertEqual((self.destination / 'manifest.json').read_bytes(),
                         (ROOT / 'manifest.json').read_bytes())
        self.assertTrue(self.launcher().is_file())
        self.assertFalse((self.destination / 'backend').exists())
        self.assertFalse(any(self.destination.parent.glob('.line-install-*')))

    def test_foreign_launcher_failure_keeps_enabled_plugin(self):
        self.launcher().parent.mkdir(parents=True)
        self.launcher().write_text('foreign content')
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('Setup complete', result.stdout)
        self.assertIn('foreign', result.stderr)
        self.assertEqual(self.launcher().read_text(), 'foreign content')
        self.assertTrue(self.destination.exists())
        self.assertIn(['omarchy', 'plugin', 'enable', 'io.github.komagata.line'], self.calls())

    def test_missing_prerequisite_before_download(self):
        (self.bin / 'secret-tool').unlink()
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('secret-tool', result.stderr)
        self.assertEqual(self.calls(), [])

    def test_existing_destination_before_tools_or_network(self):
        self.destination.mkdir(parents=True)
        (self.destination / 'sentinel').write_text('old')
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('README', result.stderr)
        self.assertEqual(self.calls(), [])
        self.assertEqual((self.destination / 'sentinel').read_text(), 'old')

    def test_symlink_parent_refused_before_network(self):
        (self.root / 'home').mkdir()
        (self.root / 'home/.config').symlink_to(self.bin, target_is_directory=True)
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_non_directory_parent_refused_before_network(self):
        (self.root / 'home').mkdir()
        (self.root / 'home/.config').write_text('keep')
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_workspace_cache_refused_before_network(self):
        self.env['GOCACHE'] = str(self.source / 'cache')
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertEqual(self.calls(), [])

    def test_old_go_refused(self):
        self.env['GO_VERSION'] = 'go1.25.9'
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertNotIn(['go', 'mod', 'download'], self.calls())

    def test_duplicate_catalog_id_refused(self):
        self.env['CATALOG'] = '[{"id":"io.github.komagata.line","sourceDir":"/other"}]'
        self.assertNotEqual(self.run_setup().returncode, 0)
        self.assertNotIn(['go', 'mod', 'download'], self.calls())

    def test_shell_discovery_failure_keeps_installed_files(self):
        self.env['PLUGINS'] = '[]'
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('Setup complete', result.stdout)
        self.assertFalse(self.launcher().exists())
        self.assertTrue(self.destination.exists())
        self.assertNotIn(['omarchy', 'plugin', 'enable', 'io.github.komagata.line'], self.calls())

    def test_failures_propagate(self):
        for failure in ('go', 'download', 'build', 'install', 'omarchy-shell', 'enable'):
            with self.subTest(failure=failure):
                self.env['FAIL'] = failure
                if self.log.exists(): self.log.unlink()
                result = self.run_setup()
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('Setup complete', result.stdout)
                self.assertFalse(self.launcher().exists())
                if failure in ('go', 'download', 'build', 'install'):
                    self.assertFalse(self.destination.exists())
                else:
                    self.assertTrue(self.destination.exists())
                    shutil.rmtree(self.destination)


if __name__ == '__main__':
    unittest.main()
