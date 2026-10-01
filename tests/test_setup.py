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
elif name == 'git' and sys.argv[1:] == ['rev-parse', '--show-toplevel']: print(os.environ.get('GIT_ROOT', os.getcwd()))
elif name == 'omarchy-plugin-catalog': print(os.environ['CATALOG'])
elif name == 'omarchy' and sys.argv[1:] == ['plugin', 'list', '--json']: print(os.environ['PLUGINS'])
if name == 'omarchy' and sys.argv[1:3] == ['plugin', 'enable'] and os.environ.get('VERIFY_READY'):
    root = Path(os.environ['GIT_ROOT'])
    assert (root / 'bin/line-gui').read_bytes() == b'\\x7fELF'
    assert (root / 'NOTICE').is_file()
    assert (Path(os.environ['XDG_DATA_HOME']) / 'applications/io.github.komagata.line.desktop').is_file()
if (os.environ['FAIL'] == 'download' and name == 'go' and sys.argv[1:] == ['mod', 'download']) or os.environ['FAIL'] == name or (os.environ['FAIL'] in ('enable', 'disable') and sys.argv[1:3] == ['plugin', os.environ['FAIL']]): sys.exit(7)
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

    def run_setup(self, *args):
        if not (self.source / 'scripts/setup').exists():
            self.fail('scripts/setup must exist for the README one-line install')
        return subprocess.run([str(self.source / 'scripts/setup'), *args], env=self.env,
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

    def test_foreign_launcher_failure_prevents_enable(self):
        self.launcher().parent.mkdir(parents=True)
        self.launcher().write_text('foreign content')
        result = self.run_setup()
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('Setup complete', result.stdout)
        self.assertIn('foreign', result.stderr)
        self.assertEqual(self.launcher().read_text(), 'foreign content')
        self.assertTrue(self.destination.exists())
        self.assertNotIn(['omarchy', 'plugin', 'enable', 'io.github.komagata.line'], self.calls())

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
                self.assertEqual(self.launcher().exists(), failure == 'enable')
                if self.launcher().exists(): self.launcher().unlink()
                if failure in ('go', 'download', 'build', 'install'):
                    self.assertFalse(self.destination.exists())
                else:
                    self.assertTrue(self.destination.exists())
                    shutil.rmtree(self.destination)

    def prepare_in_place(self):
        self.destination.parent.mkdir(parents=True)
        self.source.rename(self.destination)
        self.source = self.destination
        (self.source / '.git').mkdir()
        (self.source / '.git/config').write_text('keep git metadata')
        (self.source / 'source-sentinel').write_text('keep source')
        for name in ('package.py', 'validate_manifest.py'):
            shutil.copy2(ROOT / 'scripts' / name, self.source / 'scripts' / name)
        for name in ('BarWidget.qml', 'Panel.qml', 'Service.qml', 'manifest.json', 'LICENSE'):
            shutil.copy2(ROOT / name, self.source / name)
        shutil.copytree(ROOT / 'components', self.source / 'components')
        self.command(self.source / 'scripts/build', '''import json, os, shutil, sys
from pathlib import Path
root = Path(__file__).resolve().parents[1]
with open(os.environ['CALLS'], 'a') as log: log.write(json.dumps(['build']) + '\\n')
if os.environ['FAIL'] == 'build': sys.exit(8)
payload = root / 'build/plugin'
if payload.exists(): shutil.rmtree(payload)
(payload / 'bin').mkdir(parents=True)
for name in ('BarWidget.qml', 'Panel.qml', 'Service.qml', 'manifest.json', 'LICENSE'):
    shutil.copy2(root / name, payload / name)
shutil.copytree(root / 'components', payload / 'components')
(payload / 'NOTICE').write_text('Fictional test payload')
(payload / 'bin/line-gui').write_bytes(bytes([127, 69, 76, 70]))
(payload / 'bin/line-gui').chmod(0o755)
if os.environ['FAIL'] == 'payload': (payload / 'NOTICE').unlink()
''')
        self.env['GIT_ROOT'] = str(self.source)
        self.env['VERIFY_READY'] = '1'
        self.env['CATALOG'] = json.dumps([{'id': 'io.github.komagata.line', 'sourceDir': str(self.source)}])

    def test_in_place_success_and_repeat_preserve_source(self):
        self.prepare_in_place()
        originals = {p.relative_to(self.source): p.read_bytes()
                     for p in self.source.rglob('*') if p.is_file()}
        for attempt in range(2):
            result = self.run_setup('--in-place')
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((self.source / 'bin/line-gui').read_bytes(), b'\x7fELF')
            self.assertEqual((self.source / 'bin/line-gui').stat().st_mode & 0o777, 0o755)
            self.assertEqual((self.source / 'NOTICE').stat().st_mode & 0o777, 0o644)
            self.assertTrue(self.launcher().exists())
            calls = self.calls()
            disable = calls.index(['omarchy', 'plugin', 'disable', 'io.github.komagata.line'])
            self.assertLess(disable, calls.index(['go', 'mod', 'download']))
            self.assertEqual(calls[-1], ['omarchy', 'plugin', 'enable', 'io.github.komagata.line'])
            self.assertFalse(any(c[0] == 'install' for c in calls))
            for path, content in originals.items():
                self.assertEqual((self.source / path).read_bytes(), content)
            self.assertFalse(list(self.source.glob('.line-setup-*')))
            self.log.unlink()

    def assert_in_place_refused(self):
        result = self.run_setup('--in-place')
        self.assertNotEqual(result.returncode, 0)
        self.assertNotIn('usage:', result.stderr)
        self.assertNotIn(['go', 'mod', 'download'], self.calls())
        self.assertFalse(any(c[0] == 'build' for c in self.calls()))
        self.assertNotIn(['omarchy', 'plugin', 'disable', 'io.github.komagata.line'], self.calls())

    def test_in_place_wrong_root(self):
        self.prepare_in_place()
        foreign = self.root / 'foreign checkout'
        shutil.copytree(self.source, foreign)
        self.source = foreign
        self.assert_in_place_refused()

    def test_in_place_foreign_manifest_and_non_git_checkout(self):
        self.prepare_in_place()
        manifest = self.source / 'manifest.json'
        original = manifest.read_text()
        manifest.write_text('{"id":"foreign"}')
        self.assert_in_place_refused()
        manifest.write_text(original)
        shutil.rmtree(self.source / '.git')
        self.assert_in_place_refused()

    def test_in_place_malformed_manifest_is_reported(self):
        self.prepare_in_place()
        (self.source / 'manifest.json').write_text('[]')
        result = self.run_setup('--in-place')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('unexpected checkout manifest id', result.stderr)
        self.assertEqual(self.calls(), [])

    def test_in_place_duplicate_or_missing_catalog_identity(self):
        self.prepare_in_place()
        for source in ('/other', None, str(self.source / '..' / self.source.name)):
            with self.subTest(source=source):
                self.env['CATALOG'] = json.dumps([{'id': 'io.github.komagata.line', 'sourceDir': source}])
                self.assert_in_place_refused()

    def test_in_place_symlinks_and_wrong_types(self):
        self.prepare_in_place()
        for name in ('bin', 'bin/line-gui', 'NOTICE', 'manifest.json', '.git'):
            with self.subTest(name=name):
                path = self.source / name
                if path.exists(): path.rename(path.with_name(path.name + '.saved'))
                path.parent.mkdir(exist_ok=True)
                path.symlink_to(self.bin if name in ('bin', '.git') else self.log)
                self.assert_in_place_refused()
                path.unlink()
                saved = path.with_name(path.name + '.saved')
                if saved.exists(): saved.rename(path)
        bin_path = self.source / 'bin'
        bin_path.rmdir()
        bin_path.write_text('wrong type')
        self.assert_in_place_refused()
        bin_path.unlink()
        bin_path.mkdir()
        for name in ('bin/line-gui', 'NOTICE'):
            path = self.source / name
            path.mkdir()
            self.assert_in_place_refused()
            path.rmdir()

    def test_in_place_symlink_root_or_parent(self):
        self.prepare_in_place()
        original = self.source
        moved = self.root / 'moved'
        original.rename(moved)
        original.symlink_to(moved, target_is_directory=True)
        self.assert_in_place_refused()
        original.unlink()
        moved.rename(original)
        config = self.root / 'home/.config'
        config.rename(self.root / 'config')
        config.symlink_to(self.root / 'config', target_is_directory=True)
        self.assert_in_place_refused()

    def test_in_place_failures_never_enable(self):
        self.prepare_in_place()
        for failure in ('disable', 'download', 'build', 'payload', 'omarchy-shell'):
            with self.subTest(failure=failure):
                self.env['FAIL'] = failure
                if self.log.exists(): self.log.unlink()
                result = self.run_setup('--in-place')
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn('Setup complete', result.stdout)
                self.assertNotIn(['omarchy', 'plugin', 'enable', 'io.github.komagata.line'], self.calls())
        self.env['FAIL'] = ''
        self.launcher().parent.mkdir(parents=True)
        self.launcher().write_text('foreign')
        self.log.unlink(missing_ok=True)
        result = self.run_setup('--in-place')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('foreign', result.stderr)
        self.assertNotIn(['omarchy', 'plugin', 'enable', 'io.github.komagata.line'], self.calls())

    def test_in_place_git_root_mismatch_before_download(self):
        self.prepare_in_place()
        self.env['GIT_ROOT'] = str(self.root)
        self.assert_in_place_refused()

    def test_primary_readme_install_is_two_commands(self):
        block = (ROOT / 'README.md').read_text().split('```bash\n', 1)[1].split('```', 1)[0]
        self.assertEqual(block.splitlines(), [
            'omarchy plugin add https://github.com/komagata/line.git --yes',
            '~/.config/omarchy/plugins/io.github.komagata.line/scripts/setup --in-place'])

    def test_in_place_copy_failure_never_enables(self):
        self.prepare_in_place()
        (self.source / 'bin').mkdir()
        (self.source / 'bin/line-gui').write_bytes(b'old binary')
        (self.source / 'NOTICE').write_text('old notice')
        # Run real orchestration in-process to inject a deterministic disk copy error.
        runner = self.root / 'copy-failure.py'
        runner.write_text("import runpy, sys, shutil\n"
                          "def fail(*args, **kwargs): raise OSError('fixture copy failure')\n"
                          "shutil.copyfile = fail\n"
                          + f"sys.path.insert(0, {str(self.source / 'scripts')!r})\n"
                          + "sys.argv = ['setup.py', '--in-place']\n"
                          "runpy.run_path(sys.argv_path, run_name='__main__')\n".replace(
                              'sys.argv_path', repr(str(self.source / 'scripts/setup.py'))))
        self.env['GO_BIN'] = str(self.bin / 'go')
        result = subprocess.run([sys.executable, '-B', str(runner)], env=self.env,
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('fixture copy failure', result.stderr)
        self.assertEqual((self.source / 'bin/line-gui').read_bytes(), b'old binary')
        self.assertEqual((self.source / 'NOTICE').read_text(), 'old notice')
        self.assertIn(['omarchy', 'plugin', 'disable', 'io.github.komagata.line'], self.calls())
        self.assertNotIn(['omarchy', 'plugin', 'enable', 'io.github.komagata.line'], self.calls())
        self.assertFalse(list(self.source.glob('.line-setup-*')))


if __name__ == '__main__':
    unittest.main()
