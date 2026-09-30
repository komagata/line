#!/usr/bin/env python3
"""Launcher tests use isolated HOME, XDG paths, and fake desktop cache tools."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PLUGIN = 'io.github.komagata.line'
EXEC = f'Exec=omarchy-shell shell summon {PLUGIN}'


class LauncherTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='line-launcher-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.bin = self.root / 'bin'
        self.bin.mkdir()
        self.env = dict(os.environ, HOME=str(self.root / 'home with spaces'),
                        XDG_DATA_HOME=str(self.root / 'data with spaces'), PATH=str(self.bin))
        self.path = self.root / 'data with spaces/applications' / f'{PLUGIN}.desktop'

    def run_launcher(self, command='install'):
        return subprocess.run([sys.executable, '-B', str(ROOT / 'scripts/launcher'), command],
                              env=self.env, capture_output=True, text=True)

    def seed(self, content):
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(content)

    def test_install_and_repeat_use_xdg_path(self):
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        text = self.path.read_text()
        for field in ('[Desktop Entry]', 'Type=Application', 'Name=LINE', EXEC,
                      'Icon=internet-chat', 'Terminal=false', 'StartupNotify=false',
                      'Keywords=LINE;line;Chat;Omarchy;'):
            self.assertIn(field + '\n', text)
        self.assertEqual(self.run_launcher().returncode, 0)
        self.assertEqual(self.path.read_text(), text)
        self.assertEqual(list(self.path.parent.iterdir()), [self.path])

    def test_home_fallback_for_missing_or_empty_xdg(self):
        for xdg in (None, ''):
            with self.subTest(xdg=xdg):
                if xdg is None:
                    self.env.pop('XDG_DATA_HOME', None)
                else:
                    self.env['XDG_DATA_HOME'] = xdg
                result = self.run_launcher()
                self.assertEqual(result.returncode, 0, result.stderr)
                path = Path(self.env['HOME']) / '.local/share/applications' / self.path.name
                self.assertIn(EXEC, path.read_text())

    def test_relative_xdg_is_rejected(self):
        self.env['XDG_DATA_HOME'] = 'relative/path'
        result = self.run_launcher()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('absolute', result.stderr)

    def test_foreign_file_is_preserved_on_install_and_remove(self):
        self.seed('[Desktop Entry]\nType=Application\nName=Other\nExec=other\n')
        original = self.path.read_bytes()
        for command in ('install', 'remove'):
            result = self.run_launcher(command)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('foreign', result.stderr)
            self.assertEqual(self.path.read_bytes(), original)

    def test_existing_matching_summon_entry_can_be_updated_and_removed(self):
        old = '[Desktop Entry]\nType=Application\nName=LINE\n' + EXEC + '\nIcon=old\n'
        self.seed(old)
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Icon=internet-chat', self.path.read_text())
        self.seed(old)
        self.assertEqual(self.run_launcher('remove').returncode, 0)
        self.assertFalse(self.path.exists())

    def test_existing_entry_with_old_access_time_can_be_refreshed(self):
        self.seed('[Desktop Entry]\n' + EXEC + '\n')
        os.utime(self.path, (1, 2))
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Icon=internet-chat', self.path.read_text())

    def test_owned_marker_allows_entry_refresh(self):
        self.seed('[Desktop Entry]\nX-Omarchy-Line-Launcher=true\nExec=old\n')
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(EXEC, self.path.read_text())

    def test_similar_exec_and_action_group_do_not_establish_ownership(self):
        for content in (
            '[Desktop Entry]\n' + EXEC + ' extra\n',
            '[Desktop Entry]\nExec=other\n[Desktop Action LINE]\n' + EXEC + '\n',
        ):
            with self.subTest(content=content):
                self.seed(content)
                self.assertNotEqual(self.run_launcher().returncode, 0)
                self.assertEqual(self.path.read_text(), content)

    def test_remove_owned_entry_and_repeat(self):
        self.assertEqual(self.run_launcher().returncode, 0)
        self.assertEqual(self.run_launcher('remove').returncode, 0)
        self.assertFalse(self.path.exists())
        self.assertEqual(self.run_launcher('remove').returncode, 0)

    def test_symlink_entry_and_parent_are_refused(self):
        target = self.root / 'foreign'
        target.write_text('keep')
        self.path.parent.mkdir(parents=True)
        self.path.symlink_to(target)
        for command in ('install', 'remove'):
            self.assertNotEqual(self.run_launcher(command).returncode, 0)
            self.assertTrue(self.path.is_symlink())
            self.assertEqual(target.read_text(), 'keep')
        self.path.unlink()
        self.path.parent.rmdir()
        self.path.parent.symlink_to(self.bin, target_is_directory=True)
        for command in ('install', 'remove'):
            self.assertNotEqual(self.run_launcher(command).returncode, 0)
        self.assertEqual(list(self.bin.iterdir()), [])

    def test_directory_at_entry_is_refused(self):
        self.path.mkdir(parents=True)
        self.assertNotEqual(self.run_launcher().returncode, 0)
        self.assertNotEqual(self.run_launcher('remove').returncode, 0)
        self.assertTrue(self.path.is_dir())

    def cache_tool(self, failure=False):
        tool = self.bin / 'update-desktop-database'
        tool.write_text('#!' + sys.executable + '\nimport json, sys\nfrom pathlib import Path\n'
                        + f'Path({str(self.root / "cache-call")!r}).write_text(json.dumps(sys.argv[1:]))\n'
                        + ('sys.exit(7)\n' if failure else ''))
        tool.chmod(0o755)

    def test_optional_cache_refresh_uses_argument_path(self):
        import json
        self.cache_tool()
        self.assertEqual(self.run_launcher().returncode, 0)
        self.assertEqual(json.loads((self.root / 'cache-call').read_text()), [str(self.path.parent)])
        self.assertEqual(self.run_launcher('remove').returncode, 0)

    def test_cache_failure_reports_completed_registration(self):
        self.cache_tool(failure=True)
        result = self.run_launcher()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(self.path.exists())
        self.assertIn('warning', result.stderr)
        self.assertIn('7', result.stderr)
        self.assertEqual(self.run_launcher('remove').returncode, 0)
        self.assertFalse(self.path.exists())


if __name__ == '__main__':
    unittest.main()
