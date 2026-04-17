import type { Conversation } from '../api/client';

interface SidebarProps {
  conversations: Conversation[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete: (id: string, e: React.MouseEvent) => void;
  sidebarOpen: boolean;
  onClose: () => void;
}

function getTitle(conv: Conversation): string {
  return conv.title && conv.title.trim() ? conv.title : '新しい会話';
}

export function Sidebar({
  conversations,
  selectedId,
  onSelect,
  onNew,
  onDelete,
  sidebarOpen,
  onClose,
}: SidebarProps) {
  return (
    <>
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-[99] md:hidden"
          onClick={onClose}
        />
      )}
      <aside
        className={[
          'w-64 shrink-0 border-r border-[var(--color-border)] bg-[var(--color-bg-secondary)]',
          'flex flex-col overflow-hidden',
          'max-md:fixed max-md:inset-y-0 max-md:left-0 max-md:z-[100]',
          'max-md:transition-transform max-md:duration-200',
          sidebarOpen ? 'max-md:translate-x-0' : 'max-md:-translate-x-full',
        ].join(' ')}
      >
        <div className="px-3 py-4 border-b border-[var(--color-border)] flex flex-col gap-2.5">
          <h1 className="text-[18px] font-semibold text-[var(--color-text-bright)] font-mono">
            cc-tunnel
          </h1>
          <button
            className="w-full px-4 py-2 rounded-md bg-[var(--color-accent)] text-[#1a1b26] text-sm font-medium cursor-pointer hover:bg-[var(--color-accent-hover)] transition-colors"
            onClick={onNew}
          >
            + 新しい会話
          </button>
        </div>
        <ul className="flex-1 overflow-y-auto py-2">
          {conversations.map(conv => (
            <li
              key={conv.id}
              className={[
                'flex items-center justify-between px-3 py-2 cursor-pointer transition-colors gap-1.5',
                'hover:bg-[var(--color-bg-tertiary)]',
                conv.id === selectedId
                  ? 'bg-[var(--color-bg-tertiary)] border-l-[3px] border-[var(--color-accent)] pl-[9px]'
                  : '',
              ].join(' ')}
              onClick={() => onSelect(conv.id)}
            >
              <span className="flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-[13px] text-[var(--color-text-bright)]">
                {getTitle(conv)}
              </span>
              <button
                className="shrink-0 px-1.5 py-0.5 text-xs text-[var(--color-danger)] bg-transparent rounded cursor-pointer opacity-60 hover:opacity-100 hover:bg-[rgba(247,118,142,0.15)] transition-opacity"
                onClick={(e) => onDelete(conv.id, e)}
              >
                ×
              </button>
            </li>
          ))}
        </ul>
      </aside>
    </>
  );
}
