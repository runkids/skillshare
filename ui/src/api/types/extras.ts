// Extras types
export interface ExtraTarget {
  path: string;
  mode: string;
  flatten: boolean;
  extension?: string;  // transform extension name; presence implies copy mode
  as?: string; // single-file extra: target filename
  include?: string[];
  exclude?: string[];
  file_count: number; // source files this target syncs after include/exclude
  status: string;  // "synced" | "drift" | "modified" | "not synced" | "no source"
}

export interface Extra {
  remote?: string;
  branch?: string;
  commit?: string;
  state?: "intact" | "modified" | "interrupted" | "missing" | "error";
  name: string;
  file?: string; // single-file extra: the file in source_dir (AGENTS.md ones are shared instruction files)
  source_dir: string;
  source_type: "per-extra" | "extras_source" | "default";
  file_count: number;
  source_exists: boolean;
  targets: ExtraTarget[];
}

export interface ExtraFilterEntry {
  file: string;
  status: 'synced' | 'not_included' | 'excluded';
  reason?: string; // the exclude pattern that dropped the file
}

/** What an edit that moves files returns: the sync it ran and any old-location cleanup errors. */
export interface ExtraEditResult {
  success: boolean;
  extras?: ExtrasSyncResult[];
  prune_errors?: string[];
}

export interface ExtensionInfo {
  name: string;
  description?: string;
  builtin: boolean;
  installed: boolean;
  used_by?: string[]; // names of extras referencing this extension
}

export interface ExtraDiffItem {
  action: string;  // "create" | "update" | "prune"
  file: string;
  reason: string;
}

export interface ExtraDiffResult {
  name: string;
  target: string;
  mode: string;
  synced: boolean;
  items: ExtraDiffItem[];
}

export interface ExtrasSyncResult {
  name: string;
  targets: Array<{
    target: string;
    mode: string;
    synced: number;
    skipped: number;
    pruned: number;
    errors?: string[];
    error?: string;
  }>;
}

/** Removal keeps edited source directories and copied or transformed outputs. */
export interface ExtraRemoveResult {
  success: boolean;
  remote?: string;
  commit?: string;
  deleted_dir?: boolean;
  missing_dir?: boolean;
  kept_edited_dir?: boolean;
  kept_outputs?: string[];
  warnings?: string[];
}
