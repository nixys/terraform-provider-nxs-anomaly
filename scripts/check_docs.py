#!/usr/bin/env python3
"""Regenerate docs/wiki and fail on drift, missing examples, or untranslated text."""
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def snapshot():
    return {str(p.relative_to(ROOT)): p.read_bytes()
            for directory in ('docs', 'examples', 'wiki')
            for p in (ROOT / directory).rglob('*') if p.is_file()}


def main():
    before = snapshot()
    subprocess.run(['make', 'docs', 'wiki'], cwd=ROOT, check=True)
    after = snapshot()
    errors = [f'Stale generated content: {name}' for name in sorted(before.keys() | after.keys())
              if before.get(name) != after.get(name)]
    for kind in ('resources', 'data-sources'):
        for doc in (ROOT / 'docs' / kind).glob('*.md'):
            example = ROOT / 'examples' / kind / ('anomaly_' + doc.stem)
            filename = 'resource.tf' if kind == 'resources' else 'data-source.tf'
            if not (example / filename).is_file():
                errors.append(f'Missing example: {example / filename}')
            text = doc.read_text()
            if '## Example Usage' not in text or '## Schema' not in text:
                errors.append(f'Missing example or schema section: {doc}')
            if kind == 'resources' and ('## Import' not in text or not (example / 'import.sh').is_file()):
                errors.append(f'Missing import documentation: {doc}')
    for name, content in after.items():
        if name.endswith(('.md', '.tf', '.sh')) and re.search('[\u0400-\u04ff]', content.decode()):
            errors.append(f'Untranslated Cyrillic text: {name}')
    for doc in (ROOT / 'docs' / 'guides').glob('*.md'):
        if not re.match(r'---\n.*?page_title:', doc.read_text(), re.S):
            errors.append(f'Missing Registry guide title: {doc}')
    if errors:
        print('\n'.join(errors), file=sys.stderr)
        print('Run make docs wiki and review the regenerated files.', file=sys.stderr)
        return 1
    print('Documentation and wiki are current; examples and imports are complete.')
    return 0


if __name__ == '__main__':
    sys.exit(main())
