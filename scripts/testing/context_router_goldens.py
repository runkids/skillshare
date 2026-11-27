#!/usr/bin/env python3
"""Regenerate internal/contextrouter/testdata from the reference router script.

usage: context_router_goldens.py --router-repo <checkout of runkids/agents-context-router>
                                 [--commit v0.6.0] --out internal/contextrouter/testdata

Writes cases.json (router folders, the byte sizes the reference's `check`
prints for them, the descriptions its `list` prints, and the sizes the older
scripts in LEGACY print for the same folders) and compact.json (Markdown bodies with the link spans and the
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
# Older routers a folder may still carry: before `frontmatter` sources (0.4.0), and before
# stripFrontmatter and compactPathLinks (0.5.0).
LEGACY = ['v0.2.1', 'v0.4.0']
LINK = '[docs/run.md](../docs/run.md)'


def git(repo, *args):
    return subprocess.run(['git', '-C', repo, *args], check=True, capture_output=True).stdout


def write_tree(root, files):
    for rel, text in files.items():
        path = os.path.join(root, rel)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, 'wb') as out:
            out.write(text.encode('utf-8'))


def run_router(script, files, command, in_folder=False):
    """Run the router on one folder. Older scripts only find the folder they are copied into."""
    with tempfile.TemporaryDirectory() as tmp:
        root = os.path.realpath(tmp)
        write_tree(root, files)
        env = dict(os.environ)
        env.pop('AI_CONTEXT_CONFIG', None)
        env.pop('AI_CONTEXT_ROOT', None)
        if in_folder:
            os.makedirs(os.path.join(root, 'scripts'), exist_ok=True)
            target = os.path.join(root, 'scripts', 'ai-context.py')
            with open(script, 'rb') as source, open(target, 'wb') as out:
                out.write(source.read())
            script = target
        else:
            env['AI_CONTEXT_ROOT'] = root
        result = subprocess.run([sys.executable, '-I', script, command], cwd=root, env=env,
                                capture_output=True, text=True, encoding='utf-8')
    return result.stdout


def check_sizes(script, files, in_folder=False):
    """Sizes the router prints for one folder; mounted entries are dropped."""
    sizes = {}
    for line in run_router(script, files, 'check', in_folder).split('\n'):
        match = SIZE.match(line)
        if match and '/' not in match[1]:
            sizes[match[1]] = int(match[2])
    return sizes


def sizes_of(script, files, in_folder=False):
    cfg = json.loads(files['wiki/ai-context.json'])
    sizes = check_sizes(script, files, in_folder)
    return {'kernel': sizes.get(cfg.get('rootInstructions', 'AGENTS.md')),
            'topics': {name: sizes.get(name) for name in cfg['topics']}}


def expectation(script, files):
    cfg = json.loads(files['wiki/ai-context.json'])
    expect = sizes_of(script, files)
    # `list` prints each local topic as its name padded to 18, a space, and its description.
    listed = run_router(script, files, 'list').split('\n')
    expect['descriptions'] = {}
    expect['order'] = list(cfg['topics'])
    for name in cfg['topics']:
        prefix = f'{name:18} '
        expect['descriptions'][name] = next((line[len(prefix):] for line in listed if line.startswith(prefix)), None)
    return expect


def folder(cfg, pages, kernel=None):
    """A router folder: the config, a kernel naming every topic, a wiki index, and pages."""
    files = {'wiki/ai-context.json': json.dumps(cfg, indent=1) + '\n',
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


def size_cases(script, repo, commit, legacy):
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
    described = '---\ntype: page\ndescription: {}\n---\n# Page\nbody\n'
    forms = {
        'plain': 'From the page.', 'double': '"Quoted \\"here\\" too."  # note', 'single': "'It''s single.'",
        'comment': 'Before the note  # after', 'mapping': 'key: value', 'flow': '[a, b]', 'null': 'null',
        'folded': '>\n  folded text', 'unclosed': '"never closed', 'cjk': '\u8def\u7531\u5668\u8bf4\u660e',
    }
    pages = {f'wiki/{name}.md': described.format(value) for name, value in forms.items()}
    pages['wiki/none.md'] = '# No envelope\n'
    topics = {name: {'sources': [{'path': f'wiki/{name}.md'}]} for name in list(forms) + ['none']}
    topics['own'] = topic({'path': 'wiki/plain.md'})
    topics['heading-source'] = {'sources': [{'path': 'wiki/plain.md', 'heading': 'Page'}]}
    topics['no-sources'] = {'sources': []}
    cases.append(('descriptions', folder({'stripFrontmatter': True, 'topics': topics}, pages)))
    cases.append(('order', folder(
        {'topics': {name: topic(page) for name in ('zebra', 'alpha', 'mid')}}, {'wiki/page.md': '# Page\nbody\n'})))
    out = []
    for name, files in cases:
        cfg = json.loads(files['wiki/ai-context.json'])
        out.append({'name': name, 'compact': bool(cfg.get('compactPathLinks', False)),
                    'files': files, 'expect': expectation(script, files),
                    'legacy': {tag[1:]: sizes_of(path, files, in_folder=True) for tag, path in legacy.items()}})
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
TOKENS = [LINK, LINK, '[docs/run.md][r]', '[r]: ../docs/run.md', '[R]: <../docs/run.md> "t"', '![a](b.png)', '![', '[', ']',
          '`', '``', '```', '~~~', '> ', '>', '- ', '* ', '1. ', '2) ', '    ', '  ', ' ', '\t', '<div>', '</div>', '<!--', '-->',
          '<pre>', '</pre>', '<span a="b">', '<?', '?>', '# ', '---', '===', '***', '\\', 'text', '\u8def\u7531', '\u00e9',
          '(', ')', '"', "'", '[wiki/guide.md](guide.md)', '[x](y)', '[r]', '[r][]', '!', '<a@b.c>', '<http://x>', '|', ':']


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
            lines = [''.join(rng.choice(TOKENS) for _ in range(rng.randint(0, 8))) for _ in range(rng.randint(1, 14))]
            inputs.append((f'generated-{index:03}', '\n'.join(lines) + '\n'))
        out = []
        for name, body in inputs:
            lines = body.split('\n')
            # Offsets are characters in the text the scanner reads: the lines with tabs among
            # their leading container markers expanded. Record them as UTF-8 bytes of that text.
            leads = [module.CONTAINER_LEAD.match(line).end() for line in lines]
            text = '\n'.join(line[:lead].expandtabs(4) + line[lead:] for line, lead in zip(lines, leads))
            offset = lambda position: len(text[:position].encode('utf-8'))
            spans = [[offset(start), offset(end)] for _, _, start, end in module.link_destinations(lines, spans=True)]
            out.append({'name': name, 'source': 'wiki/guide.md', 'input': body, 'spans': spans,
                        'output': module.compact_path_links(body, 'wiki/guide.md', root=root)})
    return out


def dump(path, value):
    """One case per line: a changed case is one changed line, and span lists stay readable."""
    with open(path, 'w', encoding='utf-8', newline='\n') as out:
        out.write('{"reference": %s, "cases": [\n' % json.dumps(value['reference']))
        out.write(',\n'.join(json.dumps(case, sort_keys=True, ensure_ascii=True) for case in value['cases']))
        out.write('\n]}\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__.split('\n')[0])
    parser.add_argument('--router-repo', required=True)
    parser.add_argument('--commit', default='v0.6.0')
    parser.add_argument('--out', required=True)
    args = parser.parse_args()
    with tempfile.TemporaryDirectory() as tmp:
        script = os.path.join(tmp, 'ai-context.py')
        with open(script, 'wb') as out:
            out.write(git(args.router_repo, 'show', f'{args.commit}:{SCRIPT}'))
        legacy = {}
        for tag in LEGACY:
            legacy[tag] = os.path.join(tmp, f'ai-context-{tag}.py')
            with open(legacy[tag], 'wb') as out:
                out.write(git(args.router_repo, 'show', f'{tag}:{SCRIPT}'))
        os.makedirs(args.out, exist_ok=True)
        dump(os.path.join(args.out, 'cases.json'),
             {'reference': args.commit, 'cases': size_cases(script, args.router_repo, args.commit, legacy)})
        dump(os.path.join(args.out, 'compact.json'), {'reference': args.commit, 'cases': compact_cases(script)})


if __name__ == '__main__':
    main()
