export interface ProfileDefinition {
  name: string;
  description: string;
  /** null passes all skills through; an empty include array selects none. */
  skills: { include: string[] | null; exclude: string[] | null } | null;
  selected: number;
  unmatched: string[];
  usable: boolean;
  reason?: string;
}

export interface ProfilesResponse {
  active: string;
  state: 'none' | 'active' | 'unavailable';
  pending: boolean;
  profiles: ProfileDefinition[];
}

export interface ActiveProfileRequest {
  name: string | null;
  dryRun?: boolean;
  discardEdits?: boolean;
  allowEmpty?: boolean;
}

export interface ProfileEntryChange {
  target: string;
  entry: string;
  step: string;
  class: 'absent' | 'managed_link' | 'clean_copy' | 'modified_copy' | 'unmanaged';
  reason: string;
}

export interface ProfileTarget {
  name: string;
  state: 'applied' | 'incomplete' | 'failed';
  added: string[] | null;
  refreshed: string[] | null;
  removed: string[] | null;
  converted: string[] | null;
  kept_modified: ProfileEntryChange[] | null;
  error?: string;
  prune_error?: string;
  failed_removals?: string[];
}

export interface ProfileApplyResult {
  profile: { previous: string; active: string; pending: boolean };
  complete: boolean;
  targets: ProfileTarget[] | null;
  blocked: ProfileEntryChange[] | null;
  symlink_targets: string[] | null;
  snapshot: { target: string; path: string; restore: string; retention: string }[] | null;
}

/** On replacement, omitted skills stay unchanged; null or an empty object selects none. */
export interface ProfileDraft {
  name?: string;
  description?: string;
  skills?: { include?: string[]; exclude?: string[] } | null;
}

export interface ProfileMembershipRequest {
  add?: string[];
  remove?: string[];
}

export interface ProfileMutationResult {
  changed: boolean;
  needsSync: boolean;
  skills: { name: string; result: 'added' | 'already-in' | 'removed' | 'not-in' }[] | null;
}

export interface ProfilePreviewResult {
  selected: string[];
  unmatched: string[];
}
