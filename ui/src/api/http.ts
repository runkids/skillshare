import { BASE_PATH } from '../lib/basePath';

export const BASE = BASE_PATH + '/api';

export class ApiError extends Error {
  status: number;
  code?: string;
  params?: Record<string, unknown>;
  fallbackMessage: string;

  constructor(
    status: number,
    message: string,
    opts?: { code?: string; params?: Record<string, unknown>; fallbackMessage?: string },
  ) {
    super(message);
    this.status = status;
    this.code = opts?.code;
    this.params = opts?.params;
    this.fallbackMessage = opts?.fallbackMessage ?? message;
  }
}

interface ParsedApiError {
  code?: string;
  message: string;
  params?: Record<string, unknown>;
}

function defaultErrorCode(status: number): string {
  switch (status) {
    case 400:
      return 'bad_request';
    case 401:
    case 403:
      return 'unauthorized';
    case 404:
      return 'not_found';
    case 409:
      return 'conflict';
    case 422:
      return 'validation';
    default:
      return status >= 500 ? 'internal' : 'generic';
  }
}

export function parseApiErrorPayload(data: any, status: number, statusText: string): ParsedApiError {
  const rawError = data?.error;
  if (rawError && typeof rawError === 'object') {
    const code = typeof rawError.code === 'string' ? rawError.code : defaultErrorCode(status);
    const message = typeof rawError.message === 'string' ? rawError.message : statusText || 'Request failed';
    const params = rawError.params && typeof rawError.params === 'object'
      ? rawError.params as Record<string, unknown>
      : undefined;
    return { code, message, params };
  }

  const message = typeof rawError === 'string' ? rawError : statusText || 'Request failed';
  const code = typeof data?.error_code === 'string'
    ? data.error_code
    : typeof data?.code === 'string'
      ? data.code
      : defaultErrorCode(status);
  const params = data?.error_params && typeof data.error_params === 'object'
    ? data.error_params as Record<string, unknown>
    : data?.params && typeof data.params === 'object'
      ? data.params as Record<string, unknown>
      : undefined;
  return { code, message, params };
}

// kindQuery returns the `?kind=` query for a request, or '' when no kind is given.
export function kindQuery(kind?: string): string {
  return kind ? `?kind=${encodeURIComponent(kind)}` : '';
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    res = await fetch(BASE + path, {
      headers: { 'Content-Type': 'application/json' },
      ...init,
    });
  } catch {
    throw new ApiError(0, 'Server connection lost - try restarting with "skillshare ui".', {
      code: 'connection_lost',
    });
  }
  const text = await res.text();
  if (!text) {
    throw new ApiError(res.status || 502, 'Empty response from server (request may have timed out)', {
      code: 'empty_response',
    });
  }
  let data: any;
  try {
    data = JSON.parse(text);
  } catch {
    throw new ApiError(res.status || 502, `Invalid JSON response: ${text.slice(0, 200)}`, {
      code: 'invalid_json',
      params: { snippet: text.slice(0, 200) },
    });
  }
  if (!res.ok) {
    const parsed = parseApiErrorPayload(data, res.status, res.statusText);
    throw new ApiError(res.status, parsed.message, {
      code: parsed.code,
      params: parsed.params,
      fallbackMessage: parsed.message,
    });
  }
  return data as T;
}

// createSSEStream creates an EventSource with the standard done/error lifecycle.
// The `handlers` map registers named SSE event listeners; the special key "done"
// is treated as the terminal event that closes the connection.
export function createSSEStream(
  url: string,
  handlers: Record<string, (data: any) => void>,
  onError: (err: Error) => void,
  errorMessage: string,
): EventSource {
  const es = new EventSource(url);
  let completed = false;
  for (const [event, handler] of Object.entries(handlers)) {
    if (event === 'done') {
      es.addEventListener('done', (e) => {
        completed = true;
        es.close();
        handler(JSON.parse((e as MessageEvent).data));
      });
    } else {
      es.addEventListener(event, (e) => {
        handler(JSON.parse((e as MessageEvent).data));
      });
    }
  }
  // The server's own `error` event carries {"error": reason}; a dropped
  // connection fires a plain Event without data.
  es.addEventListener('error', (e) => {
    if (completed) return;
    es.close();
    const data = (e as MessageEvent).data;
    let reason: unknown;
    if (typeof data === 'string') {
      try { reason = JSON.parse(data)?.error; } catch { /* not JSON: use the fallback message */ }
    }
    onError(new Error(typeof reason === 'string' && reason ? reason : errorMessage));
  });
  return es;
}
