/// <reference types="node" />

import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import process from 'node:process';
import { describe, expect, it } from 'vitest';
import type { Extra } from '../api/client';
import { extraRestoreCommand } from './remoteExtras';

describe('actual restore command shell arguments', () => {
  // Compatible optional shell selection: baseline ignores it, fixed helper
  // selects quoting explicitly without changing any argument assertions.
  const commandForShell = extraRestoreCommand as (extra: Extra, project: boolean, powershell?: boolean) => string;
  for (const shell of ['posix', 'powershell']) {
    for (const branch of ['main', "release/o'brien", 'release/a\'$b`c"d']) {
      for (const project of [false, true]) {
        const unavailable = shell === 'powershell' ? !process.env.SKILLSHARE_TEST_PWSH : process.platform === 'win32';
        it.skipIf(unavailable)(`${shell}/${branch}/project=${project}`, () => {
          const extra = { name: 'rules', remote: 'example-org/hub', branch } as Extra;
          const command = commandForShell(extra, project, shell === 'powershell');
          const want = ['install', extra.remote, '--kind', 'extra', '--name', extra.name, '--branch', branch, project ? '-p' : '-g'];
          const home = mkdtempSync(join(tmpdir(), 'skillshare-shell-fixture-'));
          const env = { ...process.env, HOME: home, XDG_CONFIG_HOME: home, XDG_CACHE_HOME: home, POWERSHELL_TELEMETRY_OPTOUT: '1' };
          try {
            // Invoke only a fixture function, never the skillshare executable.
            if (shell === 'posix') {
              const out = execFileSync('sh', ['-c', 'skillshare() { printf \'%s\\0\' "$@"; }\n' + command], { encoding: 'utf8', env });
              expect(out.replace(/\0$/, '').split('\0')).toEqual(want);
            } else {
              const script = 'function skillshare { [Console]::Out.Write((ConvertTo-Json -Compress -InputObject @($args))) }\n' + command;
              const out = execFileSync(process.env.SKILLSHARE_TEST_PWSH!, ['-NoProfile', '-NonInteractive', '-Command', script], { encoding: 'utf8', env });
              expect(JSON.parse(out)).toEqual(want);
            }
          } finally {
            rmSync(home, { recursive: true, force: true });
          }
        });
      }
    }
  }
});
