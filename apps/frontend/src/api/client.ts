import createClient from 'openapi-fetch';
import type { paths, components } from './schema';

const client = createClient<paths>({ baseUrl: '/api' });

export type Conversation = components['schemas']['Conversation'];
export type ConversationDetail = components['schemas']['ConversationDetail'];
export type Message = components['schemas']['Message'];
export type CreateConversationRequest = components['schemas']['CreateConversationRequest'];
export type SendMessageRequest = components['schemas']['SendMessageRequest'];

// SSE イベントの型（バックエンドが送信するイベント）
export type SSETextEvent = { type: 'text'; content: string };
export type SSEDoneEvent = { type: 'done'; session_id: string; cost_usd: number };
export type SSEErrorEvent = { type: 'error'; message: string };
export type SSEEvent = SSETextEvent | SSEDoneEvent | SSEErrorEvent;

function throwOnError<T>(result: { data?: T; error?: unknown }): T {
  if (result.error) throw result.error;
  return result.data as T;
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
