import { describe, expect, it } from 'vitest';
import { parseApiErrorPayload } from './client';

describe('parseApiErrorPayload', () => {
  it('parses the new structured error format', () => {
    expect(parseApiErrorPayload({
      error: {
        code: 'target.not_found',
        message: 'target not found: claude',
        params: { target: 'claude' },
      },
    }, 404, 'Not Found')).toEqual({
      code: 'target.not_found',
      message: 'target not found: claude',
      params: { target: 'claude' },
    });
  });

  it('parses legacy string errors with sidecar codes', () => {
    expect(parseApiErrorPayload({
      error: 'target not found: claude',
      error_code: 'target.not_found',
      error_params: { target: 'claude' },
    }, 404, 'Not Found')).toEqual({
      code: 'target.not_found',
      message: 'target not found: claude',
      params: { target: 'claude' },
    });
  });

  it('falls back to status-derived codes for legacy errors', () => {
    expect(parseApiErrorPayload({ error: 'missing' }, 404, 'Not Found')).toEqual({
      code: 'not_found',
      message: 'missing',
      params: undefined,
    });
  });
});
