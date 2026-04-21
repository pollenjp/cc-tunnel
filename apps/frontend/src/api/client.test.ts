import { describe, it, expect } from 'vitest';

describe('client module', () => {
  it('exports conversations API functions', async () => {
    const mod = await import('./client');
    expect(mod.listConversations).toBeDefined();
    expect(mod.createConversation).toBeDefined();
    expect(mod.getConversation).toBeDefined();
    expect(mod.deleteConversation).toBeDefined();
    expect(mod.sendMessage).toBeDefined();
  });
});
