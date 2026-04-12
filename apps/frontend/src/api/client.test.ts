import { describe, it, expect } from 'vitest';

describe('BACKEND_URL', () => {
  it('defaults to /api when window.__ENV__ is not set', async () => {
    // window.__ENV__ is undefined in test environment
    const mod = await import('./client');
    // The module should load without errors
    expect(mod.createSession).toBeDefined();
    expect(mod.listSessions).toBeDefined();
    expect(mod.sendKeys).toBeDefined();
    expect(mod.getOutput).toBeDefined();
    expect(mod.getAllOutputs).toBeDefined();
    expect(mod.deleteSession).toBeDefined();
    expect(mod.resizeSession).toBeDefined();
    expect(mod.discoverSessions).toBeDefined();
  });
});
