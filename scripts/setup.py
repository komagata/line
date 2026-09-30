#!/usr/bin/env python3
"""Explicit source install; host packages and existing installs are never changed."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[1]
PLUGIN = 'io.github.komagata.line'


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, **kwargs)


def main():
    if len(sys.argv) != 1:
        raise ValueError('usage: ./scripts/setup (updates: see README.md)')
    destination = Path.home() / '.config/omarchy/plugins' / PLUGIN
    for path in (destination, *destination.parents):
        if path.is_symlink():
            raise ValueError(f'symlink destination refused: {path}')
        if path != destination and path.exists() and not path.is_dir():
            raise ValueError(f'destination parent is not a directory: {path}')
    if destination.exists():
        raise ValueError(f'destination exists: {destination}; see README.md 更新 for explicit upgrade steps')
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
    version = subprocess.check_output([go, 'env', 'GOVERSION'], env=env, text=True).strip()
    match = re.fullmatch(r'go(\d+)\.(\d+)(?:\.\d+)?', version)
    if not match or tuple(map(int, match.groups())) < (1, 26):
        raise ValueError(f'Go >= 1.26 required; found {version}')
    catalog = json.loads(subprocess.check_output(['omarchy-plugin-catalog'], text=True))
    if not isinstance(catalog, list):
        raise ValueError('invalid Omarchy plugin catalog')
    if any(item.get('id') == PLUGIN for item in catalog):
        raise ValueError('plugin id already installed; see README.md 更新 before building')
    print('Downloading pinned Go modules and building source…', flush=True)
    run([go, 'mod', 'download'], cwd=ROOT / 'backend', env=env)
    run([str(ROOT / 'scripts/build')], env=env)
    run([str(ROOT / 'scripts/install'), str(destination)], env=env)
    run(['omarchy-shell', 'shell', 'rescanPlugins'])
    for attempt in range(40):
        plugins = json.loads(subprocess.check_output(['omarchy', 'plugin', 'list', '--json'], text=True))
        if any(item.get('id') == PLUGIN for item in plugins):
            break
        time.sleep(0.05)
    else:
        raise ValueError('installed, but shell discovery failed; rescan and enable manually (README.md)')
    run(['omarchy', 'plugin', 'enable', PLUGIN])
    run([sys.executable, '-B', str(ROOT / 'scripts/launcher'), 'install'])
    print('Setup complete. Open LINE from the app launcher or the bar.')


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        sys.exit(f'setup: {error}')
