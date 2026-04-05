export type SessionType = 'claude_code' | 'multi_agent_shogun';

export interface Session {
  id: string;
  type: SessionType;
  tmux_name: string;
  pane_count: number;
  created_at: string;
}

export interface DiscoveredSession {
  type: SessionType;
  tmux_names: string[];
}

export interface OutputResponse {
  output: string;
}

export interface AllOutputsResponse {
  panes: Record<string, string>;
}

const BASE = '';

export interface CreateSessionOptions {
  type?: SessionType;
  tmux_name?: string;
  width?: number;
  height?: number;
}

export async function createSession(opts?: CreateSessionOptions): Promise<Session> {
  const res = await fetch(`${BASE}/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts ?? {}),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function listSessions(): Promise<Session[]> {
  const res = await fetch(`${BASE}/sessions`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function discoverSessions(): Promise<DiscoveredSession[]> {
  const res = await fetch(`${BASE}/sessions/discover`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function sendKeys(id: string, keys: string[], paneIndex?: number): Promise<void> {
  const params = paneIndex != null ? `?paneIndex=${paneIndex}` : '';
  const res = await fetch(`${BASE}/sessions/${id}/input${params}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keys }),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function resizeSession(id: string, width: number, height: number): Promise<void> {
  const res = await fetch(`${BASE}/sessions/${id}/resize`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ width, height }),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function getOutput(id: string, paneIndex?: number): Promise<string> {
  const params = paneIndex != null ? `?paneIndex=${paneIndex}` : '';
  const res = await fetch(`${BASE}/sessions/${id}/output${params}`);
  if (!res.ok) throw new Error(await res.text());
  const data: OutputResponse = await res.json();
  return data.output;
}

export async function getAllOutputs(id: string): Promise<Record<string, string>> {
  const res = await fetch(`${BASE}/sessions/${id}/outputs`);
  if (!res.ok) throw new Error(await res.text());
  const data: AllOutputsResponse = await res.json();
  return data.panes;
}

export async function deleteSession(id: string): Promise<void> {
  const res = await fetch(`${BASE}/sessions/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(await res.text());
}
