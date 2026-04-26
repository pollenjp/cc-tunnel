import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/react';
import { Sidebar } from './Sidebar';
import type { Conversation } from '../api/client';

function makeConv(overrides: Partial<Conversation> & { id: string }): Conversation {
  return {
    title: 'テスト会話',
    model: 'claude-sonnet-4-6',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    status: 'idle',
    ...overrides,
  };
}

const defaultProps = {
  selectedId: null,
  onSelect: vi.fn(),
  onNew: vi.fn(),
  onDelete: vi.fn(),
  sidebarOpen: true,
  onClose: vi.fn(),
};

describe('Sidebar spinner', () => {
  it('shows spinner for conversation with status=running', () => {
    const conv = makeConv({ id: 'conv-running', status: 'running', title: '実行中会話' });
    render(<Sidebar {...defaultProps} conversations={[conv]} />);

    // スピナー要素が存在すること (animate-spin クラスで識別)
    const spinners = document.querySelectorAll('.animate-spin');
    // 会話リスト中のスピナーが1つ以上あること
    expect(spinners.length).toBeGreaterThanOrEqual(1);
  });

  it('does not show spinner for conversation with status=idle', () => {
    const conv = makeConv({ id: 'conv-idle', status: 'idle', title: 'アイドル会話' });
    render(<Sidebar {...defaultProps} conversations={[conv]} />);

    // ログアウトボタンは表示されないので、スピナーは0個
    const spinners = document.querySelectorAll('.animate-spin');
    expect(spinners.length).toBe(0);
  });

  it('does not show spinner for conversation with status=completed', () => {
    const conv = makeConv({ id: 'conv-completed', status: 'completed', title: '完了会話' });
    render(<Sidebar {...defaultProps} conversations={[conv]} />);

    const spinners = document.querySelectorAll('.animate-spin');
    expect(spinners.length).toBe(0);
  });

  it('shows spinner only for running conversation in mixed list', () => {
    const convRunning = makeConv({ id: 'conv-r', status: 'running', title: '実行中' });
    const convIdle = makeConv({ id: 'conv-i', status: 'idle', title: 'アイドル' });
    const convCompleted = makeConv({ id: 'conv-c', status: 'completed', title: '完了' });

    render(
      <Sidebar {...defaultProps} conversations={[convRunning, convIdle, convCompleted]} />,
    );

    // runningの会話のみスピナーが1個表示
    const spinners = document.querySelectorAll('.animate-spin');
    expect(spinners.length).toBe(1);
  });
});
