import createClient from 'openapi-fetch';
import type { paths, components } from './schema';

const baseUrl = window.__ENV__?.BACKEND_URL ?? '/api';
const client = createClient<paths>({ baseUrl });

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

export async function getAuthStatus(conversationId: string): Promise<AuthStatus> {
  const result = await client.GET('/auth/status', { params: { query: { conversationId } } });
  return throwOnError(result);
}

export async function initiateLogin(conversationId: string, method?: string): Promise<LoginResponse> {
  const result = await client.POST('/auth/login', {
    body: method
      ? { conversationId, method: method as 'claudeai' | 'console' }
      : { conversationId },
  });
  return throwOnError(result);
}

export async function logout(conversationId: string): Promise<AuthStatus> {
  const result = await client.POST('/auth/logout', { params: { query: { conversationId } }, body: undefined });
  return throwOnError(result);
}

export async function submitAuthInput(conversationId: string, input: string): Promise<AuthInputResponse> {
  const result = await client.POST('/auth/input', { body: { conversationId, input } });
  return throwOnError(result);
}

export async function getAuthOutput(conversationId: string, since: number): Promise<AuthOutputResponse> {
  const result = await client.GET('/auth/output', { params: { query: { conversationId, since } } });
  return throwOnError(result);
}

export async function cancelLogin(conversationId: string): Promise<AuthCancelResponse> {
  const result = await client.POST('/auth/cancel', { params: { query: { conversationId } }, body: undefined });
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
