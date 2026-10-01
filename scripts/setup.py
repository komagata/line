#!/usr/bin/env python3
"""Explicit source-copy install or build in the canonical Git-managed checkout."""
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).absolute().parents[1]
PLUGIN = 'io.github.komagata.line'


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, **kwargs)


def check_path(path, *, directory=False, required=False):
    for parent in (path, *path.parents):
        if parent.is_symlink():
            raise ValueError(f'symlink path refused: {parent}')
        if parent != path and parent.exists() and not parent.is_dir():
            raise ValueError(f'path parent is not a directory: {parent}')
    try:
        mode = path.lstat().st_mode
    except FileNotFoundError:
        if required:
            raise ValueError(f'required path missing: {path}')
        return
    if not (stat.S_ISDIR(mode) if directory else stat.S_ISREG(mode)):
        raise ValueError(f'wrong path type: {path}')


def check_checkout(destination):
    check_path(destination, directory=True, required=True)
    if ROOT != destination or destination != destination.resolve():
        raise ValueError('--in-place requires the canonical installed Git checkout')
    check_path(ROOT / '.git', directory=True, required=True)
    check_path(ROOT / 'manifest.json', required=True)
    manifest = json.loads((ROOT / 'manifest.json').read_text())
    if not isinstance(manifest, dict) or manifest.get('id') != PLUGIN:
        raise ValueError('unexpected checkout manifest id')
    check_generated_paths()


def check_generated_paths():
    check_path(ROOT / 'bin', directory=True)
    check_path(ROOT / 'bin/line-gui')
    check_path(ROOT / 'NOTICE')


def install_generated():
    from package import payload
    artifact = ROOT / 'build/plugin'
    payload(artifact)
    # Stage both files before replacing either; source and Git metadata stay intact.
    with tempfile.TemporaryDirectory(prefix='.line-setup-', dir=ROOT) as temporary:
        stage = Path(temporary)
        for name, mode in (('bin/line-gui', 0o755), ('NOTICE', 0o644)):
            target = stage / Path(name).name
            shutil.copyfile(artifact / name, target)
            target.chmod(mode)
        check_generated_paths()
        (ROOT / 'bin').mkdir(exist_ok=True)
        for name in ('bin/line-gui', 'NOTICE'):
            check_generated_paths()
            os.replace(stage / Path(name).name, ROOT / name)


def main():
    if sys.argv[1:] not in ([], ['--in-place']):
        raise ValueError('usage: ./scripts/setup [--in-place] (updates: see README.md)')
    in_place = sys.argv[1:] == ['--in-place']
    destination = Path.home() / '.config/omarchy/plugins' / PLUGIN
    if in_place:
        check_checkout(destination)
    else:
        check_path(destination, directory=True)
        if destination.exists():
            raise ValueError(f'destination exists: {destination}; see README.md for explicit upgrade steps')
    go = os.environ.get('GO_BIN', '')
    if not go or not os.access(go, os.X_OK):
        raise ValueError('required tool missing: go (Go >= 1.26)')
    for tool in ('git', 'secret-tool', 'omarchy', 'omarchy-shell', 'omarchy-plugin-catalog'):
        if not shutil.which(tool):
            raise ValueError(f'required tool missing: {tool}; see README.md prerequisites')
    env = dict(os.environ, GO_BIN=go, GOTOOLCHAIN='local')
    env['GOCACHE'] = env.get('GOCACHE', '/tmp/omarchy-line-gocache')
    if Path(env['GOCACHE']).resolve().is_relative_to(ROOT):
        raise ValueError('GOCACHE must be outside source workspace')
    if in_place:
        top = subprocess.check_output(['git', 'rev-parse', '--show-toplevel'],
                                      cwd=ROOT, text=True).strip()
        if top != str(ROOT):
            raise ValueError('--in-place requires the checkout Git root')
    version = subprocess.check_output([go, 'env', 'GOVERSION'], env=env, text=True).strip()
    match = re.fullmatch(r'go(\d+)\.(\d+)(?:\.\d+)?', version)
    if not match or tuple(map(int, match.groups())) < (1, 26):
        raise ValueError(f'Go >= 1.26 required; found {version}')
    catalog = json.loads(subprocess.check_output(['omarchy-plugin-catalog'], text=True))
    if not isinstance(catalog, list) or any(not isinstance(item, dict) for item in catalog):
        raise ValueError('invalid Omarchy plugin catalog')
    for item in catalog:
        if item.get('id') == PLUGIN and (not in_place or item.get('sourceDir') != str(ROOT)):
            raise ValueError('plugin id already installed or missing checkout identity; see README.md before building')
    if in_place:
        run(['omarchy', 'plugin', 'disable', PLUGIN])
    print('Downloading pinned Go modules and building source…', flush=True)
    run([go, 'mod', 'download'], cwd=ROOT / 'backend', env=env)
    run([str(ROOT / 'scripts/build')], env=env)
    if in_place:
        install_generated()
    else:
        run([str(ROOT / 'scripts/install'), str(destination)], env=env)
    run(['omarchy-shell', 'shell', 'rescanPlugins'])
    for attempt in range(40):
        plugins = json.loads(subprocess.check_output(['omarchy', 'plugin', 'list', '--json'], text=True))
        if any(item.get('id') == PLUGIN for item in plugins):
            break
        time.sleep(0.05)
    else:
        raise ValueError('installed, but shell discovery failed; rescan and enable manually (README.md)')
    run([sys.executable, '-B', str(ROOT / 'scripts/launcher'), 'install'])
    run(['omarchy', 'plugin', 'enable', PLUGIN])
    print('Setup complete. Open LINE from the app launcher or the bar.')


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        sys.exit(f'setup: {error}')
