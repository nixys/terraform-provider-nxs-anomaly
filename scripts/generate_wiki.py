#!/usr/bin/env python3
"""Build a flat, portable wiki from the provider's versioned documentation."""
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[1]
DEST = ROOT / 'wiki'
REPO = 'https://github.com/nixys/terraform-provider-nxs-anomaly/blob/main/'


def render():
    sources = {ROOT / 'docs/index.md': 'Provider.md',
               ROOT / 'CONTRIBUTING.md': 'Development.md',
               ROOT / 'RELEASING.md': 'Releasing.md'}
    for kind, prefix in [('resources', 'Resource'), ('data-sources', 'Data-Source'), ('guides', 'Guide')]:
        sources.update({p: f'{prefix}-{p.stem}.md' for p in sorted((ROOT / 'docs' / kind).glob('*.md'))})
    DEST.mkdir(exist_ok=True)
    pages = {}
    for source, name in sources.items():
        content = re.sub(r'\A---\n.*?\n---\n+', '', source.read_text(), count=1, flags=re.S)

        def link(match):
            label, target = match.groups()
            if target.startswith(('#', 'https://', 'http://', 'mailto:')):
                return match.group(0)
            path, sep, anchor = target.partition('#')
            resolved = (source.parent / path).resolve()
            url = sources.get(resolved)
            if url is None:
                try:
                    url = REPO + str(resolved.relative_to(ROOT))
                except ValueError:
                    return match.group(0)
            return f'[{label}]({url}{sep}{anchor})'

        pages[name] = re.sub(r'\[([^\]]+)\]\(([^)]+)\)', link, content)
    nav = ['# nxs-anomaly Terraform Provider', '',
           'Manage alerting and on-call configuration with Terraform.', '',
           '- [Provider configuration](Provider.md)',
           '- [Usage and examples](Guide-usage.md)',
           '- [Development](Development.md)',
           '- [Registry release procedure](Releasing.md)', '', '## Resources', '']
    nav += [f'- [anomaly_{p.stem}](Resource-{p.stem}.md)' for p in sorted((ROOT / 'docs/resources').glob('*.md'))]
    nav += ['', '## Data Sources', '']
    nav += [f'- [anomaly_{p.stem}](Data-Source-{p.stem}.md)' for p in sorted((ROOT / 'docs/data-sources').glob('*.md'))]
    pages['Home.md'] = '\n'.join(nav) + '\n'
    pages['_Sidebar.md'] = '[Home](Home.md)\n\n' + '\n'.join(nav[4:]) + '\n'
    # This directory contains generated pages only.
    for stale in DEST.glob('*.md'):
        if stale.name not in pages:
            stale.unlink()
    for name, content in pages.items():
        (DEST / name).write_text(content)
    print(f'Generated {len(pages)} wiki pages in {DEST}')


if __name__ == '__main__':
    render()
