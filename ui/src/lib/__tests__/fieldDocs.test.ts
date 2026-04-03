import { describe, it, expect } from 'vitest';
import { fieldDocs } from '../fieldDocs';

describe('fieldDocs', () => {
  it('has entries for all top-level config fields', () => {
    const required = ['sync_mode', 'target_naming', 'targets', 'extras'];
    for (const key of required) {
      expect(fieldDocs[key]).toBeDefined();
    }
  });

  it('documents target_naming values at both global and target scope', () => {
    expect(fieldDocs.target_naming?.allowedValues).toEqual(['flat', 'standard']);
    expect(fieldDocs['targets.skills.target_naming']?.allowedValues).toEqual(['flat', 'standard']);
  });

  it('every entry has description, type, and example', () => {
    for (const [key, doc] of Object.entries(fieldDocs)) {
      expect(doc.description, `${key}.description`).toBeTruthy();
      expect(doc.type, `${key}.type`).toBeTruthy();
      expect(doc.example, `${key}.example`).toBeTruthy();
    }
  });

  it('fields with allowedValues have non-empty arrays', () => {
    for (const [key, doc] of Object.entries(fieldDocs)) {
      if (doc.allowedValues) {
        expect(doc.allowedValues.length, `${key}.allowedValues`).toBeGreaterThan(0);
      }
    }
  });
});
