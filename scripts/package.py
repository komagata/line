#!/usr/bin/env python3
"""Development-only offline packaging and reversible, explicitly requested install."""
import argparse
import ctypes
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tempfile
import uuid

ROOT = Path(__file__).resolve().parents[1]
ROOT_QML = ('BarWidget.qml', 'Panel.qml', 'Service.qml')


def run(argv, **kwargs):
    return subprocess.check_output(argv, text=True, **kwargs)


def plain_tree(path):
    for parent in (path, *path.parents):
        if parent.is_symlink():
            raise ValueError(f'symlink path refused: {parent}')
    if not path.is_dir():
        raise ValueError(f'not a directory: {path}')
    for item in path.rglob('*'):
        mode = item.lstat().st_mode
        if not (stat.S_ISREG(mode) or stat.S_ISDIR(mode)):
            raise ValueError(f'special entry refused: {item}')


def payload(path):
    plain_tree(path)
    expected = set(ROOT_QML) | {'manifest.json', 'LICENSE', 'NOTICE', 'bin/line-gui'}
    expected |= {str(p.relative_to(ROOT)) for p in (ROOT / 'components').iterdir()
                 if p.suffix in ('.qml', '.js')}
    actual = {str(p.relative_to(path)) for p in path.rglob('*') if p.is_file()}
    if actual != expected:
        raise ValueError(f'unexpected payload: missing={expected-actual}, extra={actual-expected}')
    executable = path / 'bin/line-gui'
    if not os.access(executable, os.X_OK) or executable.read_bytes()[:4] != b'\x7fELF':
        raise ValueError('line-gui must be executable ELF')
    run(['python3', str(ROOT / 'scripts/validate_manifest.py'), str(path)])


def notices(env, version):
    text = (ROOT / 'docs/go-source-provenance.txt').read_text()
    text += f'\nPlugin version: {version}\nGo build ID: line-gui-{version}\n'
    text += '\nUPSTREAM LICENSE (backend/LICENSE)\n' + (ROOT / 'backend/LICENSE').read_text()
    goroot = Path(run([env['GO_BIN'], 'env', 'GOROOT'], env=env).strip())
    text += '\nGO STANDARD LIBRARY LICENSE\n' + (goroot / 'LICENSE').read_text()
    dependencies = run([env['GO_BIN'], 'list', '-mod=readonly', '-deps', '-f',
                        '{{if and .Module (not .Module.Main)}}{{.Module.Path}}\t{{.Module.Version}}\t{{.Module.Dir}}{{end}}',
                        './cmd/line-gui'], cwd=ROOT / 'backend', env=env)
    for entry in sorted(set(dependencies.splitlines()) - {''}):
        module, revision, directory = entry.split('\t')
        text += f'\nDEPENDENCY {module} {revision}\n'
        found = False
        for name in ('LICENSE', 'LICENSE.txt', 'LICENSE.md', 'COPYING', 'NOTICE'):
            license_path = Path(directory) / name
            if license_path.is_file():
                text += f'\n{name}\n' + license_path.read_text()
                found = True
        if not found:
            raise ValueError(f'missing license for {module} {revision}')
    return text


def build(args):
    plain_tree(ROOT / 'components')
    env = dict(os.environ, CGO_ENABLED='0', GOOS='linux', GOPROXY='off', GOTOOLCHAIN='local')
    env['GOCACHE'] = env.get('GOCACHE', '/tmp/omarchy-line-gocache')
    cache = Path(env['GOCACHE']).resolve()
    if cache.is_relative_to(ROOT):
        raise ValueError('GOCACHE must be outside source workspace')
    version = json.loads((ROOT / 'manifest.json').read_text())['version']
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._+-]*', version):
        raise ValueError('unsafe manifest version')
    output = ROOT / 'build'
    if output.is_symlink():
        raise ValueError('symlink build directory refused')
    output.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.plugin-stage-', dir=output) as temporary:
        stage = Path(temporary) / 'plugin'
        (stage / 'bin').mkdir(parents=True)
        (stage / 'components').mkdir()
        for name in (*ROOT_QML, 'manifest.json', 'LICENSE'):
            if not stat.S_ISREG((ROOT / name).lstat().st_mode):
                raise ValueError(f'non-regular source file: {name}')
            shutil.copyfile(ROOT / name, stage / name)
        for item in sorted((ROOT / 'components').iterdir()):
            if item.suffix in ('.qml', '.js'):
                shutil.copyfile(item, stage / 'components' / item.name)
        (stage / 'NOTICE').write_text(notices(env, version))
        subprocess.run([env['GO_BIN'], 'build', '-mod=readonly', '-buildvcs=false', '-trimpath',
                        '-ldflags', f'-s -w -buildid=line-gui-{version}',
                        '-o', str(stage / 'bin/line-gui'), './cmd/line-gui'],
                       cwd=ROOT / 'backend', env=env, check=True)
        payload(stage)
        target = output / 'plugin'
        if target.exists() or target.is_symlink():
            payload(target)
            shutil.rmtree(target)
        stage.rename(target)
    executable = target / 'bin/line-gui'
    print(f'Plugin: {target}\nVersion: {version}\nBinary bytes: {executable.stat().st_size}')
    print(f'SHA-256: {hashlib.sha256(executable.read_bytes()).hexdigest()}')
    print(f'Payload bytes: {sum(p.stat().st_size for p in target.rglob("*") if p.is_file())}')


def rename_new(source, destination):
    # Linux atomic no-clobber rename; refuse unsupported platforms before moving.
    libc = ctypes.CDLL(None, use_errno=True)
    rename = getattr(libc, 'renameat2', None)
    if rename is None:
        raise ValueError('install requires Linux renameat2')
    rename.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_int, ctypes.c_char_p, ctypes.c_uint]
    rename.restype = ctypes.c_int
    if rename(-100, os.fsencode(source), -100, os.fsencode(destination), 1) != 0:
        error = ctypes.get_errno()
        raise OSError(error, os.strerror(error), str(destination))


def installed_plugin(path):
    plain_tree(path)
    manifest = json.loads((path / 'manifest.json').read_text())
    if manifest.get('id') != 'io.github.komagata.line':
        raise ValueError('existing directory belongs to another plugin')


def install(args):
    destination = Path(os.path.abspath(Path(args.destination).expanduser()))
    if destination.name != 'io.github.komagata.line':
        raise ValueError('destination basename must be io.github.komagata.line')
    for parent in (destination, *destination.parents):
        if parent.is_symlink():
            raise ValueError(f'symlink path refused: {parent}')
    backup_root = destination.parent.with_name(destination.parent.name + '-backups')
    for parent in (backup_root, *backup_root.parents):
        if parent.is_symlink():
            raise ValueError(f'symlink backup path refused: {parent}')
    if args.rollback:
        source = Path(os.path.abspath(Path(args.rollback).expanduser()))
        if source.parent != backup_root or not source.name.startswith(destination.name + '.backup-'):
            raise ValueError('rollback source must be in the dedicated backup directory created by this script')
        installed_plugin(source)
        installed_plugin(destination)
        backup_root.mkdir(parents=True, exist_ok=True)
        backup = backup_root / (destination.name + '.backup-' + uuid.uuid4().hex)
        rename_new(destination, backup)
        try:
            rename_new(source, destination)
        except BaseException:
            rename_new(backup, destination)
            raise
        print(f'Restored: {destination}\nReplaced version preserved: {backup}')
        return
    source = ROOT / 'build/plugin'
    payload(source)
    exists = destination.exists()
    if exists and not args.upgrade:
        raise ValueError('destination exists; disable plugin and use --upgrade explicitly')
    if exists:
        installed_plugin(destination)
    elif args.upgrade:
        raise ValueError('--upgrade requires an existing plugin')
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='.line-install-', dir=destination.parent) as temporary:
        stage = Path(temporary) / 'plugin'
        shutil.copytree(source, stage)
        payload(stage)
        backup = None
        if exists:
            backup_root.mkdir(parents=True, exist_ok=True)
            backup = backup_root / (destination.name + '.backup-' + uuid.uuid4().hex)
            rename_new(destination, backup)
        try:
            # Never silently replace a path created after the initial refusal check.
            if destination.exists() or destination.is_symlink():
                raise ValueError('destination appeared during staging')
            rename_new(stage, destination)
        except BaseException:
            if backup is not None and not destination.exists():
                rename_new(backup, destination)
            raise
    print(f'Installed files: {destination}')
    if backup:
        print(f'Previous version preserved: {backup}\nRollback: scripts/install {destination} --rollback {backup}')
    print('No shell reload, plugin enablement, account CLI or credential changes performed.')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    commands.add_parser('build')
    installer = commands.add_parser('install')
    installer.add_argument('destination')
    mode = installer.add_mutually_exclusive_group()
    mode.add_argument('--upgrade', action='store_true')
    mode.add_argument('--rollback', metavar='BACKUP')
    args = parser.parse_args()
    try:
        (build if args.command == 'build' else install)(args)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        parser.exit(1, f'package: {error}\n')


if __name__ == '__main__':
    main()
