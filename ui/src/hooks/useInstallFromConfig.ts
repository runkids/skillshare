import { useCallback, useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, isConnectionLost } from '../api/client';
import type { ConfigEntry, InstallFromConfigResult } from '../api/client';
import { queryKeys, staleTimes } from '../lib/queryKeys';
import { invalidate } from '../lib/queryEvents';
import { plural, useT } from '../i18n';

export type ConfigInstallPhase = 'idle' | 'running' | 'done' | 'closing';

const COLLAPSE_MS = 450;

/**
 * Installs what config records but the disk lacks, like bare `skillshare install`.
 * While a run is shown, `entries` stays the list it started from so rows can show their results.
 */
export function useInstallFromConfig() {
  const queryClient = useQueryClient();
  const { data } = useQuery({
    queryKey: queryKeys.missingConfigEntries,
    queryFn: () => api.missingConfigEntries(),
    staleTime: staleTimes.missingConfigEntries,
  });
  const [phase, setPhase] = useState<ConfigInstallPhase>('idle');
  const [snapshot, setSnapshot] = useState<ConfigEntry[]>([]);
  const [result, setResult] = useState<InstallFromConfigResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);

  const live = data?.entries ?? [];
  const entries = phase === 'idle' ? live : snapshot;
  const t = useT();
  const counts = useEntryCounts();
  const done = phase === 'done' || phase === 'closing';
  const repoSkills = useCountUp(result?.installedRepoSkills ?? 0, done);
  const failed = new Set(result?.failed ?? []);
  const installedRepos = result?.installedRepos ?? 0;
  const installed = result?.installed ?? 0;
  const repos = entries.filter((e) => e.tracked).length;
  /** What a finished run did: "Installed 1 tracked repo (12 skills), 1 failed". */
  const summary = [
    installed > 0 &&
      t('install.fromConfig.installed', {
        items: counts(
          installedRepos,
          installed - installedRepos,
          installedRepos > 0 ? repoSkills : undefined,
        ),
      }),
    failed.size > 0 && t('install.fromConfig.failed', { count: failed.size }),
  ]
    .filter(Boolean)
    .join(', ');
  /** What the entries hold: "1 tracked repo, 2 skills". */
  const contents = counts(repos, entries.length - repos);

  const run = useCallback(async () => {
    setSnapshot(live);
    setPhase('running');
    setError(null);
    try {
      const r = await api.installFromConfig();
      setResult(r);
      setPhase('done');
      void invalidate(queryClient, 'configInstalled');
    } catch (err) {
      if (isConnectionLost(err)) {
        setError(t('common.connectionLost'));
        void invalidate(queryClient, 'configInstalled');
      } else {
        setError((err as Error).message);
      }
      setPhase('idle');
    }
  }, [live, queryClient, t]);

  /** Collapses the finished panel, then hands the list back to the query. */
  const dismiss = useCallback(() => {
    setPhase('closing');
    timer.current = setTimeout(() => {
      setPhase('idle');
      setResult(null);
    }, COLLAPSE_MS);
  }, []);

  return {
    entries,
    file: data?.file ?? '.metadata.json',
    phase,
    done,
    failed,
    summary,
    contents,
    error,
    run,
    dismiss,
  };
}

/** Counts up from 0 to `target` once `active` turns on, for a summary number that fills in. */
function useCountUp(target: number, active: boolean) {
  const [value, setValue] = useState(0);
  useEffect(() => {
    if (!active) {
      setValue(0);
      return;
    }
    if (target <= 0 || window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
      setValue(target);
      return;
    }
    const step = Math.max(1, Math.ceil(target / 24));
    const id = setInterval(
      () =>
        setValue((v) => {
          const next = Math.min(target, v + step);
          if (next === target) clearInterval(id);
          return next;
        }),
      40,
    );
    return () => clearInterval(id);
  }, [target, active]);
  return value;
}

/** "1 tracked repo, 2 skills": what a list of config entries holds. */
function useEntryCounts() {
  const t = useT();
  return (repos: number, skills: number, repoSkills?: number) => {
    const parts: string[] = [];
    if (repos > 0) {
      const label = t(plural('install.fromConfig.repos', repos), { count: repos });
      parts.push(
        repoSkills === undefined
          ? label
          : `${label} (${t(plural('install.fromConfig.skills', repoSkills), { count: repoSkills })})`,
      );
    }
    if (skills > 0) parts.push(t(plural('install.fromConfig.skills', skills), { count: skills }));
    return parts.join(', ');
  };
}
