export interface HookEditorHandlerValue {
  type: 'command' | 'http' | 'prompt' | 'agent';
  command: string;
  url: string;
  prompt: string;
  timeout: string;
  statusMessage: string;
}

export interface HookEditorValue {
  tool: string;
  event: string;
  matcher: string;
  handlers: HookEditorHandlerValue[];
}

function emptyHandler(): HookEditorHandlerValue {
  return {
    type: 'command',
    command: '',
    url: '',
    prompt: '',
    timeout: '',
    statusMessage: '',
  };
}

export function createEmptyHookEditorValue(): HookEditorValue {
  return {
    tool: '',
    event: '',
    matcher: '',
    handlers: [emptyHandler()],
  };
}

export function createEmptyHookEditorHandlerValue(): HookEditorHandlerValue {
  return emptyHandler();
}
