import createClient from 'openapi-fetch';
import type { paths, components } from './schema';

const client = createClient<paths>({ baseUrl: '/api' });

export type AuthStatus = components['schemas']['AuthStatus'];
export type LoginRequest = components['schemas']['LoginRequest'];
export type LoginResponse = components['schemas']['LoginResponse'];
export type AuthInputRequest = components['schemas']['AuthInputRequest'];
export type AuthInputResponse = components['schemas']['AuthInputResponse'];
export type AuthOutputResponse = components['schemas']['AuthOutputResponse'];
export type AuthCancelResponse = components['schemas']['AuthCancelResponse'];
export type Conversation = components['schemas']['Conversation'];
export type ConversationDetail = components['schemas']['ConversationDetail'];
export type Message = components['schemas']['Message'];
export type CreateConversationRequest = components['schemas']['CreateConversationRequest'];
export type SendMessageRequest = components['schemas']['SendMessageRequest'];
export type SendMessageResponse = components['schemas']['SendMessageResponse'];
export type ToolCallData = components['schemas']['ToolCallData'];

function throwOnError<T>(result: { data?: T; error?: unknown }): T {
  if (result.error) throw result.error;
  return result.data as T;
}

export async function getAuthStatus(): Promise<AuthStatus> {
  const result = await client.GET('/auth/status');
  return throwOnError(result);
}

export async function initiateLogin(method?: string): Promise<LoginResponse> {
  const result = await client.POST('/auth/login', {
    body: method ? { method: method as 'claudeai' | 'console' } : {},
  });
  return throwOnError(result);
}

export async function logout(): Promise<AuthStatus> {
  const result = await client.POST('/auth/logout', { body: undefined });
  return throwOnError(result);
}

export async function submitAuthInput(input: string): Promise<AuthInputResponse> {
  const result = await client.POST('/auth/input', { body: { input } });
  return throwOnError(result);
}

export async function getAuthOutput(since: number): Promise<AuthOutputResponse> {
  const result = await client.GET('/auth/output', { params: { query: { since } } });
  return throwOnError(result);
}

export async function cancelLogin(): Promise<AuthCancelResponse> {
  const result = await client.POST('/auth/cancel', { body: undefined });
  return throwOnError(result);
}

export async function listConversations(): Promise<Conversation[]> {
  const result = await client.GET('/conversations');
  return throwOnError(result) ?? [];
}

export async function createConversation(
  req?: CreateConversationRequest,
): Promise<Conversation> {
  const result = await client.POST('/conversations', { body: (req ?? {}) as CreateConversationRequest });
  return throwOnError(result);
}

export async function getConversation(id: string): Promise<ConversationDetail> {
  const result = await client.GET('/conversations/{conversationId}', {
    params: { path: { conversationId: id } },
  });
  return throwOnError(result);
}

export async function deleteConversation(id: string): Promise<void> {
  await client.DELETE('/conversations/{conversationId}', {
    params: { path: { conversationId: id } },
  });
}

export async function sendMessage(
  conversationId: string,
  content: string,
): Promise<{ message_id: string }> {
  const res = await fetch(`/api/conversations/${conversationId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  });
  if (!res.ok) throw new Error(`sendMessage failed: ${res.status}`);
  return res.json();
}
