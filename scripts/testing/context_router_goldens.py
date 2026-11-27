#!/usr/bin/env python3
"""Regenerate internal/contextrouter/testdata from the reference router script.

usage: context_router_goldens.py --router-repo <checkout of runkids/agents-context-router>
                                 [--commit 19c94f9] --out internal/contextrouter/testdata

Writes cases.json (router folders and the byte sizes the reference's `check`
prints for them) and compact.json (Markdown bodies with the link spans and the
compactPathLinks output the reference computes). Nothing in those files is
edited by hand: change a case here and rerun.

The fixtures live inside JSON on purpose. Loose AGENTS.md and wiki/ trees under
testdata/ would be flagged by this repository's own `scripts/ai-context.py check`,
and JSON strings keep \\r, a BOM and control characters intact through Git.
"""
import argparse, importlib.util, io, json, os, random, re, subprocess, sys, tarfile, tempfile

SCRIPT = 'skills/agents-context-router/scripts/ai-context.py'
ROUTED = 'skills/agents-context-router/evals/fixtures/routed/'
SIZE = re.compile(r'^(.+?)\s+(\d+) B  \(cap \d+\)$')
LINK = '[docs/run.md](../docs/run.md)'


def git(repo, *args):
    return subprocess.run(['git', '-C', repo, *args], check=True, capture_output=True).stdout


def write_tree(root, files):
    for rel, text in files.items():
        path = os.path.join(root, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, 'wb') as out:
            out.write(text.encode('utf-8'))


def check_sizes(script, files):
    """Sizes the reference prints for one folder; mounted entries are dropped."""
    with tempfile.TemporaryDirectory() as tmp:
        root = os.path.realpath(tmp)
        write_tree(root, files)
        env = dict(os.environ, AI_CONTEXT_ROOT=root)
        env.pop('AI_CONTEXT_CONFIG', None)
        result = subprocess.run([sys.executable, '-I', script, 'check'], cwd=root, env=env,
                                capture_output=True, text=True, encoding='utf-8')
    sizes = {}
    for line in result.stdout.split('\n'):
        match = SIZE.match(line)
        if match and '/' not in match[1]:
            sizes[match[1]] = int(match[2])
    return sizes


def expectation(script, files):
    cfg = json.loads(files['wiki/ai-context.json'])
    sizes = check_sizes(script, files)
    return {'kernel': sizes.get(cfg.get('rootInstructions', 'AGENTS.md')),
            'topics': {name: sizes.get(name) for name in cfg['topics']}}


def folder(cfg, pages, kernel=None):
    """A router folder: the config, a kernel naming every topic, a wiki index, and pages."""
    files = {'wiki/ai-context.json': json.dumps(cfg, indent=1, sort_keys=True) + '\n',
             'wiki/README.md': '# Topics\n'}
    if kernel is None:
        kernel = '# Kernel\n\n' + ''.join(f'- `{name}`\n' for name in cfg['topics'])
    if kernel is not False:
        files[cfg.get('rootInstructions', 'AGENTS.md')] = kernel
    files.update(pages)
    return files


def topic(*sources, **extra):
    return dict({'description': 'Fixture topic.', 'sources': list(sources)}, **extra)


def routed(repo, commit):
    archive = tarfile.open(fileobj=io.BytesIO(git(repo, 'archive', commit, ROUTED)))
    files = {}
    for member in archive.getmembers():
        rel = member.name[len(ROUTED):]
        if member.isfile() and not rel.startswith('scripts/'):
            files[rel] = archive.extractfile(member).read().decode('utf-8')
    return files


COMPACT_PAGE = (
    '# Guide\n'
    f'| Ship it | {LINK} |\n'
    f'- {LINK} and more.\n'
    '[./docs/run.md](../docs/run.md) [docs//run.md](../docs/run.md)\n'
    '[run](../docs/run.md)\n'
    '[docs/run.md](../docs/run.md#step)\n'
    '[docs/run.md](../docs/run.md "Title")\n'
    '![docs/run.md](../docs/run.md)\n'
    '[docs/run.md][ref]\n\n[ref]: ../docs/run.md\n'
    '![outer [docs/run.md](../docs/run.md)](../docs/run.md)\n'
    '[https://x.dev/a](https://x.dev/a)\n'
    '[/docs/run.md](/docs/run.md)\n'
    '```\n' + LINK + '\n```\n'
    f'`{LINK}`\n'
    f'`before`{LINK}\n{LINK}`after`\n'
    f'`before`{LINK}{LINK}`after`\n'
    f'{LINK}{LINK}\n'
    f'>\t{LINK}\n'
    f'> {LINK}\n'
    f'- item\n  > {LINK}\n'
    '# Other\n'
    f'tail {LINK}\n')


def size_cases(script, repo, commit):
    page = {'path': 'wiki/page.md'}
    cases = [('routed', routed(repo, commit))]
    cases.append(('newlines', folder(
        {'topics': {'cr': topic(page)}}, {'wiki/page.md': '# Page\rone\rtwo\r'},
        kernel='# Kernel\r\n\r\n- `cr`\r\n')))
    enveloped = '\ufeff---\ntype: page\n...\t\t\n\n# Page\nbody\n## Part\npart\n'
    cases.append(('envelope', folder(
        {'stripFrontmatter': True, 'topics': {
            'plain': topic(page),
            'kept': topic({'path': 'wiki/page.md', 'stripFrontmatter': False}),
            'part': topic({'path': 'wiki/page.md', 'heading': 'Part', 'frontmatter': True}),
            'unclosed': topic({'path': 'wiki/open.md'})}},
        {'wiki/page.md': enveloped, 'wiki/open.md': '---\ntype: page\n\n# Open\nbody\n'})))
    cases.append(('headings', folder(
        {'topics': {
            'fenced': topic({'path': 'wiki/page.md', 'heading': 'Real'}),
            'children': topic({'path': 'wiki/page.md', 'heading': 'Parent'}),
            'closed': topic({'path': 'wiki/page.md', 'heading': 'Closed'}),
            'twice': topic({'path': 'wiki/page.md', 'heading': 'Twice'})}},
        {'wiki/page.md': '```sh\n# Real\n```\n# Real\nreal\n# Parent\n## Child\nchild\n### Deep\ndeep\n'
                         '# Closed ##\nclosed\n## Twice\na\n## Twice\nb\n'})))
    cases.append(('repeat', folder({'topics': {'twice': topic(page, page)}}, {'wiki/page.md': '# Page\nbody\n'})))
    cases.append(('good-then-bad', folder(
        {'topics': {'broken': topic(page, {'path': 'wiki/page.md', 'heading': 'Gone'})}},
        {'wiki/page.md': '# Page\nbody\n'})))
    cases.append(('cjk', folder({'topics': {'cjk': topic(page)}}, {'wiki/page.md': '# 路由\n上下文路由器。\n'})))
    cases.append(('ctrl', folder({'topics': {'ctrl': topic(page)}}, {'wiki/page.md': '# Page\nbody\x1c\n'})))
    cases.append(('unlisted', folder(
        {'topics': {'named': topic(page), 'hidden': topic(page)}}, {'wiki/page.md': '# Page\nbody\n'},
        kernel='# Kernel\n\n- `named`\n')))
    cases.append(('missing-kernel', folder(
        {'topics': {'page': topic(page)}}, {'wiki/page.md': '# Page\nbody\n'}, kernel=False)))
    edge = {'topics': {'equal': topic(page), 'over': topic(page)}}
    measured = check_sizes(script, folder(edge, {'wiki/page.md': '# Page\nbody\n'}))['equal']
    edge['topics']['equal']['maxBytes'] = measured
    edge['topics']['over']['maxBytes'] = measured - 1
    cases.append(('cap-edge', folder(edge, {'wiki/page.md': '# Page\nbody\n'})))
    inner = folder({'topics': {'guide': topic({'path': 'wiki/guide.md'})}}, {'wiki/guide.md': '# Guide\n'})
    mounted = folder({'mounts': {'kb': {'root': 'modules/kb'}}, 'topics': {'page': topic(page)}},
                     {'wiki/page.md': '# Page\nbody\n'})
    mounted.update({'modules/kb/' + rel: text for rel, text in inner.items()})
    cases.append(('mounts', mounted))
    cases.append(('compact-basic', folder(
        {'compactPathLinks': True, 'stripFrontmatter': True, 'topics': {
            'guide': topic({'path': 'wiki/guide.md'}),
            'part': topic({'path': 'wiki/guide.md', 'heading': 'Guide'}, {'path': 'docs/run.md'})}},
        {'wiki/guide.md': '---\ntype: page\n---\n' + COMPACT_PAGE, 'docs/run.md': 'Run.\n'})))
    out = []
    for name, files in cases:
        cfg = json.loads(files['wiki/ai-context.json'])
        out.append({'name': name, 'compact': bool(cfg.get('compactPathLinks', False)),
                    'files': files, 'expect': expectation(script, files)})
    return out


HAND = {
    'paragraph': f'See {LINK} now.\n',
    'table-row': f'| Ship it | {LINK} |\n',
    'indented-code': f'para\n\n    {LINK}\n',
    'lazy-continuation': f'para\n    {LINK}\n',
    'fence-backticks': f'```\n{LINK}\n```\n{LINK}\n',
    'fence-tildes': f'~~~\n{LINK}\n~~~\n{LINK}\n',
    'fence-unclosed': f'```\n{LINK}\n',
    'fence-info-with-backtick': f'``` a`b\n{LINK}\n',
    'html-block': f'<div>\n{LINK}\n\n{LINK}\n',
    'html-pre': f'<pre>\n{LINK}\n\n{LINK}\n</pre>\n{LINK}\n',
    'html-comment': f'<!--\n{LINK}\n-->\n{LINK}\n',
    'html-inline': f'a <span title="{LINK}">x</span> {LINK}\n',
    'autolink': f'<https://x.dev/a> {LINK}\n',
    'image': '![docs/run.md](../docs/run.md)\n',
    'link-in-image-text': f'![outer {LINK}](image.png)\n',
    'reference-link': '[docs/run.md][r]\n\n[r]: ../docs/run.md\n',
    'reference-shortcut-around': f'[a {LINK} b][r]\n\n[r]: ../docs/run.md\n',
    'definition-title-next-line': f'[r]: ../docs/run.md\n  "Title"\n{LINK} [x][r]\n',
    'code-span': f'`{LINK}` {LINK}\n',
    'code-span-double': f'`` {LINK} ` `` {LINK}\n',
    'backtick-unclosed': f'` {LINK}\n',
    'escaped-bracket': '\\[docs/run.md](../docs/run.md)\n',
    'quote': f'> {LINK}\n',
    'quote-tab': f'>\t{LINK}\n',
    'list-item': f'- {LINK}\n',
    'list-in-quote': f'> - {LINK}\n>   {LINK}\n',
    'quote-in-list': f'- item\n  > {LINK}\n',
    'nested-list-lazy': f'- a\n  - b\n{LINK}\n',
    'list-code': f'- a\n\n      {LINK}\n',
    'setext': f'{LINK}\n===\n{LINK}\n---\n',
    'thematic': f'***\n{LINK}\n',
    'after-code-span': f'`before`{LINK}\n',
    'before-code-span': f'{LINK}`after`\n',
    'back-to-back': f'{LINK}{LINK}\n',
    'dot-slash': '[./docs/run.md](../docs/run.md)\n',
    'double-slash': '[docs//run.md](../docs/run.md)\n',
    'title': '[docs/run.md](../docs/run.md "Title")\n',
    'fragment': '[docs/run.md](../docs/run.md#step)\n',
    'absolute': '[/docs/run.md](/docs/run.md)\n',
    'other-file': '[docs/run.md](run.md)\n',
    'nested-brackets': f'[a [b] {LINK}](x.md)\n',
    'cjk-before': f'路由 {LINK} 器\n',
}
TOKENS = [LINK, '[docs/run.md][r]', '[r]: ../docs/run.md', '![a](b.png)', '![', '[', ']', '`', '``', '```', '~~~',
          '> ', '- ', '1. ', '    ', '\t', '<div>', '</div>', '<!--', '-->', '<pre>', '# ', '---', '\\', 'text', ' ']


def compact_cases(script):
    with tempfile.TemporaryDirectory() as tmp:
        root = os.path.realpath(tmp)
        os.environ['AI_CONTEXT_ROOT'] = root
        os.environ.pop('AI_CONTEXT_CONFIG', None)
        spec = importlib.util.spec_from_file_location('ai_context_reference', script)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        rng = random.Random(19)
        inputs = list(HAND.items())
        for index in range(300):
            lines = [''.join(rng.choice(TOKENS) for _ in range(rng.randint(0, 6))) for _ in range(rng.randint(1, 12))]
            inputs.append((f'generated-{index:03}', '\n'.join(lines) + '\n'))
        out = []
        for name, body in inputs:
            lines = body.split('\n')
            text = module.markdown_text(lines)[0]
            offset = lambda position: len(text[:position].encode('utf-8'))
            spans = [[offset(start), offset(end)] for _, _, start, end in module.link_destinations(lines, spans=True)]
            out.append({'name': name, 'source': 'wiki/guide.md', 'input': body, 'spans': spans,
                        'output': module.compact_path_links(body, 'wiki/guide.md', root=root)})
    return out


def dump(path, value):
    with open(path, 'w', encoding='utf-8', newline='\n') as out:
        json.dump(value, out, indent=1, sort_keys=True, ensure_ascii=True)
        out.write('\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__.split('\n')[0])
    parser.add_argument('--router-repo', required=True)
    parser.add_argument('--commit', default='19c94f9')
    parser.add_argument('--out', required=True)
    args = parser.parse_args()
    with tempfile.TemporaryDirectory() as tmp:
        script = os.path.join(tmp, 'ai-context.py')
        with open(script, 'wb') as out:
            out.write(git(args.router_repo, 'show', f'{args.commit}:{SCRIPT}'))
        os.makedirs(args.out, exist_ok=True)
        dump(os.path.join(args.out, 'cases.json'),
             {'reference': args.commit, 'cases': size_cases(script, args.router_repo, args.commit)})
        dump(os.path.join(args.out, 'compact.json'), {'reference': args.commit, 'cases': compact_cases(script)})


if __name__ == '__main__':
    main()
