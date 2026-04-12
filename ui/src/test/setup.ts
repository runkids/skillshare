import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

const localStorageData = new Map<string, string>();

Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: {
    get length() {
      return localStorageData.size;
    },
    clear() {
      localStorageData.clear();
    },
    getItem(key: string) {
      return localStorageData.has(key) ? localStorageData.get(key) ?? null : null;
    },
    key(index: number) {
      return Array.from(localStorageData.keys())[index] ?? null;
    },
    removeItem(key: string) {
      localStorageData.delete(key);
    },
    setItem(key: string, value: string) {
      localStorageData.set(key, value);
    },
  },
});

afterEach(() => {
  cleanup();
  localStorageData.clear();
});
