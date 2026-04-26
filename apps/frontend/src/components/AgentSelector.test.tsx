import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/react';
import { AgentSelector } from './AgentSelector';

describe('AgentSelector', () => {
  it('3つのAgentが表示されること', () => {
    render(<AgentSelector onSelect={vi.fn()} />);
    expect(screen.getByTestId('agent-btn-claude-code')).toBeTruthy();
    expect(screen.getByTestId('agent-btn-github-copilot')).toBeTruthy();
    expect(screen.getByTestId('agent-btn-cursor-cli')).toBeTruthy();
  });

  it('Claude Code は選択可能（ボタン enabled）', () => {
    render(<AgentSelector onSelect={vi.fn()} />);
    const btn = screen.getByTestId('agent-btn-claude-code');
    expect((btn as HTMLButtonElement).disabled).toBe(false);
  });

  it('GitHub Copilot はグレーアウト（ボタン disabled）', () => {
    render(<AgentSelector onSelect={vi.fn()} />);
    const btn = screen.getByTestId('agent-btn-github-copilot');
    expect((btn as HTMLButtonElement).disabled).toBe(true);
  });

  it('Cursor CLI はグレーアウト（ボタン disabled）', () => {
    render(<AgentSelector onSelect={vi.fn()} />);
    const btn = screen.getByTestId('agent-btn-cursor-cli');
    expect((btn as HTMLButtonElement).disabled).toBe(true);
  });

  it('Claude Code 選択時に onSelect("claude-code") が呼ばれること', () => {
    const onSelect = vi.fn();
    render(<AgentSelector onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('agent-btn-claude-code'));
    expect(onSelect).toHaveBeenCalledWith('claude-code');
    expect(onSelect).toHaveBeenCalledTimes(1);
  });
});
