import { afterEach, expect, it, vi } from 'vitest';
import { createSSEStream } from './http';

class FakeEventSource extends EventTarget {
  static last: FakeEventSource;
  close = vi.fn();
  constructor() {
    super();
    FakeEventSource.last = this;
  }
}

afterEach(() => vi.unstubAllGlobals());

it("reports the server's error event with its message", () => {
  vi.stubGlobal('EventSource', FakeEventSource);
  const onError = vi.fn();
  createSSEStream('/api/x/stream', {}, onError, 'Stream failed');

  FakeEventSource.last.dispatchEvent(new MessageEvent('error', { data: JSON.stringify({ error: 'doctor check failed: exit status 2' }) }));

  expect(onError).toHaveBeenCalledWith(new Error('doctor check failed: exit status 2'));
});

it('reports a dropped connection with the fallback message', () => {
  vi.stubGlobal('EventSource', FakeEventSource);
  const onError = vi.fn();
  createSSEStream('/api/x/stream', {}, onError, 'Stream failed');

  FakeEventSource.last.dispatchEvent(new Event('error'));

  expect(onError).toHaveBeenCalledWith(new Error('Stream failed'));
});
