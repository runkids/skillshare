# Hub Builder E2E Runbook

Verify author drafts, portable export, and installation by a separate recipient.

**Origin**: Hub builder — skills-only authoring without hand-editing JSON.

## Scope

- Import, save, reload, optimistic revision checks, and local-source export blockers.
- Preserve extension fields and duplicate display names.
- Export a v1 index without author paths; search and install it in another ssenv.
- The recipient installation downloads the public built-in skill from GitHub.

## Environment

Run inside the devcontainer in a fresh `ssenv` created with `--init`.
When invoking mdproof from an active ssenv, unset inherited `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`, and `SKILLSHARE_CONFIG` for mdproof so its per-runbook HOME isolation also controls configuration.
The test creates and deletes its own recipient environment and stops its own API server.
It does not publish a Hub or modify the normal development configuration.

## Steps

### 1. Author, export, and consume a Hub

```bash
python3 - <<'PY'
import json, os, pathlib, signal, socket, subprocess, time, urllib.request, urllib.error

home = pathlib.Path.home()
with socket.socket() as probe:
    probe.bind(('127.0.0.1', 0))
    port = probe.getsockname()[1]
base = f'http://127.0.0.1:{port}/api/hub/drafts'
log = open(home / 'hub-builder-server.log', 'w')
server = subprocess.Popen(['ss', 'ui', '-g', '--no-open', '--port', str(port)], stdout=log, stderr=log, start_new_session=True)
recipient = 'hub-recipient-' + str(os.getpid())
recipient_created = False

def request(method, suffix='', data=None, status=200):
    body = None if data is None else json.dumps(data).encode()
    req = urllib.request.Request(base + suffix, data=body, method=method, headers={'Content-Type': 'application/json'})
    try:
        with urllib.request.urlopen(req) as response:
            assert response.status == status
            return json.load(response)
    except urllib.error.HTTPError as error:
        assert error.code == status, (error.code, error.read().decode())
        return None

try:
    for attempt in range(120):
        try:
            request('GET')
            break
        except urllib.error.URLError:
            if server.poll() is not None:
                raise RuntimeError('API server exited; inspect hub-builder-server.log')
            time.sleep(0.25)
    else:
        raise RuntimeError('API server did not start')

    source = 'github.com/runkids/skillshare/skills/skillshare'
    imported = request('POST', '/import', {
        'schemaVersion': 1, 'sourcePath': str(home / 'author-only'), 'extension': {'kept': True},
        'skills': [
            {'name': 'Shared skill', 'source': source, 'futureField': 42},
            {'name': 'Shared skill', 'source': 'local/notes'}
        ]
    })
    draft = imported['draft']
    assert len(draft['entries']) == 2
    assert draft['entries'][0]['id'] != draft['entries'][1]['id']
    path = '/' + draft['id']
    assert any(p['code'] == 'local_source' for p in imported['problems'])
    request('POST', path + '/export', {'revision': draft['revision']}, 400)
    old = json.loads(json.dumps(draft))
    draft['entries'] = [draft['entries'][0]]
    saved = request('PUT', path, draft)
    request('PUT', path, old, 409)
    restored = request('GET', path)
    assert restored['draft'] == saved['draft']
    exported = request('POST', path + '/export', {'revision': saved['draft']['revision']})
    assert exported['schemaVersion'] == 1 and 'sourcePath' not in exported
    assert exported['extension']['kept'] and exported['skills'][0]['futureField'] == 42
    assert 'revision' not in exported and 'entries' not in exported
    print('PASS: drafts, blockers, revisions, extension fields, and portable export')

    subprocess.run(['ssenv', 'create', recipient], check=True)
    recipient_created = True
    subprocess.run(['ssenv', 'enter', recipient, '--', 'ss', 'init', '-g', '--all-targets', '--no-git', '--no-skill'], check=True)
    payload = json.dumps(exported)
    consumer = r'''
import json, pathlib, subprocess, sys
path = pathlib.Path.home() / 'skillshare-hub.json'
path.write_text(sys.argv[1])
rows = json.loads(subprocess.check_output(['ss', 'search', '--hub', str(path), '--json']))
assert len(rows) == 1 and rows[0]['Name'] == 'Shared skill'
subprocess.run(['ss', 'install', rows[0]['Source'], '--name', 'hub-builder-received', '-g'], check=True)
assert (pathlib.Path.home() / '.config/skillshare/skills/hub-builder-received/SKILL.md').is_file()
print('PASS: separate recipient searched and installed exported source')
'''
    subprocess.run(['ssenv', 'enter', recipient, '--', 'python3', '-c', consumer, payload], check=True)
    request('DELETE', path + '?revision=' + saved['draft']['revision'])
    request('GET', path, status=404)
    print('PASS: draft deleted without removing installed skills')
finally:
    os.killpg(server.pid, signal.SIGTERM)
    server.wait(timeout=10)
    log.close()
    if recipient_created:
        subprocess.run(['ssenv', 'delete', recipient, '--force'], check=True)
PY
```

Expected:
- exit_code: 0
- PASS: drafts, blockers, revisions, extension fields, and portable export
- PASS: separate recipient searched and installed exported source
- PASS: draft deleted without removing installed skills

## Browser Verification

Against an isolated author dashboard:

1. Open **Skills → My Hubs**, create a draft, select both a local skill and a remote skill.
2. Save; the local entry remains visible and export is blocked.
3. Edit a field and navigate away; cancel the discard dialog and confirm edits remain.
4. Remove the local entry, save, download, and inspect `skillshare-hub.json`.
5. Reload the page and reopen the saved draft.
6. Open the same draft in another tab; save there, then verify an older save gets a conflict without discarding edits.
7. Import an exported index; confirm custom fields and optional selectors survive.
8. Confirm the delete dialog can be cancelled, then delete the test draft.

## Pass Criteria

The automated step passes, and browser verification confirms editing, navigation protection, export, and deletion.
