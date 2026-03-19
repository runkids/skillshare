import { describe, it, expect } from 'vitest';
import { auditFieldDocs } from '../auditFieldDocs';

describe('auditFieldDocs', () => {
  it('has entries for all rule fields', () => {
    const required = ['rules', 'rules.id', 'rules.severity', 'rules.pattern',
      'rules.message', 'rules.regex', 'rules.exclude', 'rules.enabled'];
    for (const key of required) {
      expect(auditFieldDocs[key], `missing ${key}`).toBeDefined();
    }
  });

  it('every entry has description, type, and example', () => {
    for (const [key, doc] of Object.entries(auditFieldDocs)) {
      expect(doc.description, `${key}.description`).toBeTruthy();
      expect(doc.type, `${key}.type`).toBeTruthy();
      expect(doc.example, `${key}.example`).toBeTruthy();
    }
  });
});
