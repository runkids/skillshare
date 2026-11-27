import { http } from './http';

/** Why a folder is listed: the project the dashboard runs in, a source folder or one above it, or a registered project. */
export type RouterOrigin = 'current' | 'skills_source' | 'agents_source' | 'project';

/** A folder that has `wiki/ai-context.json`. `path` is the physical path. */
export interface RouterRoot {
  path: string;
  name: string;
  origin: RouterOrigin;
}

/** Why a file the router config names could not be measured. */
export type RouterFileError = 'missing' | 'not_file' | 'outside' | 'too_large' | 'invalid_utf8';

/** The file the config names as root instructions. `bytes` is null when it could not be read. */
export interface RouterKernel {
  path: string;
  bytes: number | null;
  cap: number;
  error?: RouterFileError;
}

/** One source of a topic. `bytes` is its body alone; `count` is how many headings matched when `error` is `heading`. */
export interface RouterSource {
  path: string;
  heading?: string;
  bytes: number | null;
  error?: RouterFileError | 'heading';
  count?: number;
}

/** `bytes` is null when the router would print no size either: a source failed, or the report hit its limits. `listed` is null when the kernel could not be read. */
export interface RouterTopic {
  name: string;
  description: string;
  bytes: number | null;
  cap: number;
  reason?: 'source' | 'limit';
  listed: boolean | null;
  sources: RouterSource[];
}

/** A mounted router the config declares. Listed as written; nothing under it is measured. */
export interface RouterMount {
  name: string;
  root: string;
}

/** One folder's router. With `error`, the config could not be used and there is no kernel and no topic. `reference` is the agents-context-router commit the sizes follow. */
export interface RouterReport {
  root: string;
  config: string;
  reference: string;
  error?: { code: 'config_unreadable' | 'config_invalid'; detail?: string };
  kernel?: RouterKernel;
  topics: RouterTopic[];
  truncated: boolean;
  mounts: RouterMount[];
}

/** `router` is the `selected` folder measured, or null when no folder has a router. */
export interface ContextRouterResponse {
  roots: RouterRoot[];
  selected: string;
  router: RouterReport | null;
}

export const contextRouterApi = {
  /** Read-only. `root` picks one of the listed folders; a folder that is not listed rejects with 404. */
  get: (root: string, signal?: AbortSignal) =>
    http.get<ContextRouterResponse>(
      root ? `/context/router?root=${encodeURIComponent(root)}` : '/context/router',
      signal,
    ),
};
