import createClient from 'openapi-fetch';
import type { paths, components } from './schema.d';

const client = createClient<paths>({ baseUrl: window.__ENV__?.BACKEND_URL ?? '/api' });

export type AppUser = components['schemas']['AppUser'];
export type AppAuthLoginResponse = components['schemas']['AppAuthLoginResponse'];
export type AppAuthMeResponse = components['schemas']['AppAuthMeResponse'];

function throwOnError<T>(result: { data?: T; error?: unknown }): T {
  if (result.error) throw result.error;
  return result.data as T;
}

export async function login(username: string): Promise<AppAuthLoginResponse> {
  const result = await client.POST('/app-auth/login', { body: { username } });
  return throwOnError(result);
}

export async function getMe(token: string): Promise<AppAuthMeResponse> {
  const result = await client.GET('/app-auth/me', {
    headers: { Authorization: `Bearer ${token}` },
  });
  return throwOnError(result);
}

export async function logout(token: string): Promise<void> {
  await client.POST('/app-auth/logout', {
    headers: { Authorization: `Bearer ${token}` },
  });
}

export async function updateMe(token: string, nickname: string): Promise<AppAuthMeResponse> {
  const result = await client.PATCH('/app-auth/me', {
    headers: { Authorization: `Bearer ${token}` },
    body: { nickname },
  });
  return throwOnError(result);
}
