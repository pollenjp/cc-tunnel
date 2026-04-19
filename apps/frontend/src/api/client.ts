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
export type ToolCallData = components['schemas']['ToolCallData'];

// SSE イベントの型（openapi.yaml から生成）
export type SSETextEvent = components['schemas']['SSETextEvent'];
export type SSEThinkingEvent = components['schemas']['SSEThinkingEvent'];
export type SSETextDeltaEvent = components['schemas']['SSETextDeltaEvent'];
export type SSEThinkingDeltaEvent = components['schemas']['SSEThinkingDeltaEvent'];
export type SSEToolUseStartEvent = components['schemas']['SSEToolUseStartEvent'];
export type SSEToolInputDeltaEvent = components['schemas']['SSEToolInputDeltaEvent'];
export type SSEToolResultEvent = components['schemas']['SSEToolResultEvent'];
export type SSEInitEvent = components['schemas']['SSEInitEvent'];
export type SSEHookEvent = components['schemas']['SSEHookEvent'];
export type SSERateLimitEvent = components['schemas']['SSERateLimitEvent'];
export type SSECostEvent = components['schemas']['SSECostEvent'];
export type SSEDoneEvent = components['schemas']['SSEDoneEvent'];
export type SSEErrorEvent = components['schemas']['SSEErrorEvent'];

export type SSEEvent =
  | SSETextEvent
  | SSEThinkingEvent
  | SSETextDeltaEvent
  | SSEThinkingDeltaEvent
  | SSEToolUseStartEvent
  | SSEToolInputDeltaEvent
  | SSEToolResultEvent
  | SSEInitEvent
  | SSEHookEvent
  | SSERateLimitEvent
  | SSECostEvent
  | SSEDoneEvent
  | SSEErrorEvent;

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

// SSE ストリーミングでメッセージを送信する
// onEvent: 各SSEイベントで呼ばれるコールバック
export async function sendMessage(
  conversationId: string,
  content: string,
  onEvent: (event: SSEEvent) => void,
): Promise<void> {
  const res = await fetch(`/api/conversations/${conversationId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  });

  if (!res.ok) {
    throw new Error(`sendMessage failed: ${res.status}`);
  }

  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });

    // SSE形式: "data: {...}\n\n" をパース
    const parts = buffer.split('\n\n');
    buffer = parts.pop() ?? '';

    for (const part of parts) {
      const line = part.trim();
      if (line.startsWith('data: ')) {
        try {
          const event = JSON.parse(line.slice(6)) as SSEEvent;
          onEvent(event);
        } catch {
          // パース失敗は無視
        }
      }
    }
  }
}
