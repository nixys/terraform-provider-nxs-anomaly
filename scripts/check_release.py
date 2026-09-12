#!/usr/bin/env python3
"""Validate Terraform Registry packages locally and before publishing a release."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys
import zipfile

PREFIX = 'terraform-provider-nxs-anomaly'
TARGETS = {(os, arch) for os in ('linux', 'darwin', 'windows') for arch in ('amd64', 'arm64')}


def check(directory, allow_unsigned=False):
    sums = list(directory.glob(f'{PREFIX}_*_SHA256SUMS'))
    if len(sums) != 1:
        raise ValueError('Expected exactly one SHA256SUMS file')
    checksum = sums[0]
    version = checksum.name[len(PREFIX) + 1:-len('_SHA256SUMS')]
    if not allow_unsigned and not re.fullmatch(r'(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?', version):
        raise ValueError(f'Invalid release version: {version}')
    hashes = {}
    for line in checksum.read_text().splitlines():
        digest, name = line.split(maxsplit=1)
        name = name.removeprefix('*')
        if not re.fullmatch(r'[0-9a-fA-F]{64}', digest) or Path(name).name != name or name in hashes:
            raise ValueError(f'Invalid checksum entry: {name}')
        asset = directory / name
        if not asset.is_file() or hashlib.sha256(asset.read_bytes()).hexdigest() != digest.lower():
            raise ValueError(f'Missing asset or checksum mismatch: {name}')
        hashes[name] = digest
    manifest = f'{PREFIX}_{version}_manifest.json'
    required = {manifest} | {f'{PREFIX}_{version}_{os}_{arch}.zip' for os, arch in TARGETS}
    if set(hashes) != required:
        raise ValueError(f'Unexpected asset matrix: missing={required - hashes.keys()}, extra={hashes.keys() - required}')
    data = json.loads((directory / manifest).read_text())
    if data.get('version') != 1 or data.get('metadata', {}).get('protocol_versions') != ['6.0']:
        raise ValueError('Manifest must declare version 1 and protocol 6.0')
    for os, arch in sorted(TARGETS):
        archive = directory / f'{PREFIX}_{version}_{os}_{arch}.zip'
        binary = f'{PREFIX}_v{version}' + ('.exe' if os == 'windows' else '')
        with zipfile.ZipFile(archive) as package:
            names = package.namelist()
            binaries = [name for name in names if name.startswith(PREFIX)]
            if binaries != [binary] or package.getinfo(binary).file_size == 0:
                raise ValueError(f'Invalid binary layout: {archive.name}')
            if any(Path(name).is_absolute() or '..' in Path(name).parts for name in names):
                raise ValueError(f'Unsafe archive path: {archive.name}')
            if package.testzip() is not None:
                raise ValueError(f'Corrupt ZIP: {archive.name}')
    if not allow_unsigned:
        signature = directory / (checksum.name + '.sig')
        if not signature.is_file() or signature.read_bytes().startswith(b'-----BEGIN'):
            raise ValueError('Missing binary detached GPG signature')
        subprocess.run(['gpg', '--verify', str(signature), str(checksum)], check=True)
    print(f'Validated {version}: 6 packages, manifest, checksums' + (' (unsigned rehearsal).' if allow_unsigned else ', and GPG signature.'))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    parser.add_argument('--allow-unsigned', action='store_true', help='For local snapshot rehearsals only')
    args = parser.parse_args()
    try:
        check(args.directory, args.allow_unsigned)
    except (ValueError, OSError, subprocess.CalledProcessError, zipfile.BadZipFile) as error:
        print(f'Release validation failed: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
