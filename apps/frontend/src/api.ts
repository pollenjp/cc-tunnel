export interface Session {
  id: string;
  tmux_name: string;
  created_at: string;
}

export interface OutputResponse {
  output: string;
}

const BASE = '';

export async function createSession(): Promise<Session> {
  const res = await fetch(`${BASE}/sessions`, { method: 'POST' });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function listSessions(): Promise<Session[]> {
  const res = await fetch(`${BASE}/sessions`);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function sendKeys(id: string, keys: string[]): Promise<void> {
  const res = await fetch(`${BASE}/sessions/${id}/input`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keys }),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function getOutput(id: string): Promise<string> {
  const res = await fetch(`${BASE}/sessions/${id}/output`);
  if (!res.ok) throw new Error(await res.text());
  const data: OutputResponse = await res.json();
  return data.output;
}

export async function deleteSession(id: string): Promise<void> {
  const res = await fetch(`${BASE}/sessions/${id}`, { method: 'DELETE' });
  if (!res.ok) throw new Error(await res.text());
}
