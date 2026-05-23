export interface CredentialsStatus {
  registered: boolean;
  isValid: boolean;
}

const backendBase = () => window.__ENV__?.BACKEND_URL ?? '/api';

async function apiFetch<T>(path: string, token: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${backendBase()}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...options?.headers,
    },
  });
  if (!res.ok) {
    if (res.status === 401) {
      sessionStorage.removeItem('app_auth_token');
      if (window.location.pathname !== '/login') {
        const { pathname, search, hash } = window.location;
        const redirect = `${pathname}${search}${hash}`;
        window.location.assign(`/login?${new URLSearchParams({ redirect }).toString()}`);
      }
    }
    const text = await res.text().catch(() => String(res.status));
    throw new Error(`${path} failed (${res.status}): ${text}`);
  }
  return res.json() as Promise<T>;
}

export async function getCredentialsStatus(token: string): Promise<CredentialsStatus> {
  return apiFetch<CredentialsStatus>('/credentials/status', token, { method: 'GET' });
}

export async function startRelogin(token: string, conversationId: string): Promise<{ ready: boolean }> {
  return apiFetch<{ ready: boolean }>('/credentials/relogin/start', token, {
    method: 'POST',
    body: JSON.stringify({ conversationId }),
  });
}

export async function finalizeRelogin(token: string, conversationId: string): Promise<CredentialsStatus> {
  return apiFetch<CredentialsStatus>('/credentials/relogin/finalize', token, {
    method: 'POST',
    body: JSON.stringify({ conversationId }),
  });
}
