import { create } from 'zustand';
import type { Conversation } from '../api/client';
import {
  listConversations,
  createConversation as apiCreateConversation,
  deleteConversation as apiDeleteConversation,
} from '../api/client';

interface ConversationsState {
  conversations: Conversation[];

  // selectors
  hasRunning: () => boolean;

  // actions
  refresh: () => Promise<void>;
  create: () => Promise<Conversation>;
  remove: (id: string) => Promise<void>;
  // optimistic update used when the user just sent a message; the real
  // status will be picked up by the next refresh().
  markRunning: (id: string) => void;
}

// Single conversation list store. Replaces the previous
// `useState<Conversation[]>` in ChatPage and the `onSendStart` callback
// drilled into ChatView for optimistic running-status updates.
export const useConversationsStore = create<ConversationsState>((set, get) => ({
  conversations: [],

  hasRunning: () => get().conversations.some(c => c.status === 'running'),

  refresh: async () => {
    try {
      const list = await listConversations();
      set({ conversations: list ?? [] });
    } catch (e) {
      console.error('Failed to list conversations:', e);
    }
  },

  create: async () => {
    const conv = await apiCreateConversation();
    await get().refresh();
    return conv;
  },

  remove: async (id) => {
    await apiDeleteConversation(id);
    await get().refresh();
  },

  markRunning: (id) => {
    set(state => ({
      conversations: state.conversations.map(c =>
        c.id === id ? { ...c, status: 'running' } : c,
      ),
    }));
  },
}));
