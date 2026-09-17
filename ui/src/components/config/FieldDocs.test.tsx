import { render, screen, cleanup } from '@testing-library/react';
import { afterEach, expect, it } from 'vitest';
import FieldDocs from './FieldDocs';
import { fieldDocs } from '../../lib/fieldDocs';

afterEach(cleanup);

it.each([
  ['mcp', 'mcp'],
  ['sources.mcp', 'sources.mcp'],
  ['mcp.servers.targets', 'mcp.servers'],
  ['mcp.servers.targets.url', 'mcp.servers.url'],
  ['mcp.servers.docs.env.URL.fromEnv', 'mcp.servers.env.fromEnv'],
  ['mcp.servers.docs.headers.Authorization.fromEnv', 'mcp.servers.headers.fromEnv'],
  ['mcp.servers.docs.bearerToken.fromEnv', 'mcp.servers.bearerToken.fromEnv'],
])('documents %s', (path, key) => {
  render(<FieldDocs fieldPath={path} />);
  expect(screen.getByText(fieldDocs[key].description)).toBeTruthy();
  expect(screen.queryByText('Unknown field')).toBeNull();
});
