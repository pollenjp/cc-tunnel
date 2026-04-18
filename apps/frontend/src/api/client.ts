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

// SSE イベントの型（バックエンドが送信するイベント）
export type SSETextEvent = { type: 'text'; content: string };
export type SSEThinkingEvent = { type: 'thinking'; content: string };
export type SSEDoneEvent = { type: 'done'; session_id: string; cost_usd: number };
export type SSEErrorEvent = { type: 'error'; message: string };

// デルタイベント（リアルタイム表示用）
export type SSETextDeltaEvent    = { type: 'text_delta';     content: string };
export type SSEThinkingDeltaEvent = { type: 'thinking_delta'; content: string };

// メタ情報イベント
export type SSEInitEvent = {
  type: 'init';
  model: string;
  session_id: string;
  tools?: string[];
};
export type SSERateLimitEvent = {
  type: 'rate_limit';
  status: string;
  resets_at: number;
  rate_limit_type: string;
};
export type SSECostEvent = {
  type: 'cost';
  total_cost_usd: number;
  duration_ms: number;
};

export type SSEHookEvent = {
  type: 'hook_event';
  subtype: string;
  hook_id?: string;
  hook_name?: string;
  hook_event?: string;
  session_id?: string;
};

export type SSEToolUseStartEvent = {
  type: 'tool_use_start';
  index: number;
  tool_use_id: string;
  tool_name: string;
};
export type SSEToolInputDeltaEvent = {
  type: 'tool_input_delta';
  index: number;
  partial_json: string;
};
export type SSEToolResultEvent = {
  type: 'tool_result';
  tool_use_id: string;
  content: string;
};

export type SSEEvent =
  | SSETextEvent
  | SSEThinkingEvent
  | SSETextDeltaEvent
  | SSEThinkingDeltaEvent
  | SSEDoneEvent
  | SSEErrorEvent
  | SSEInitEvent
  | SSERateLimitEvent
  | SSECostEvent
  | SSEHookEvent
  | SSEToolUseStartEvent
  | SSEToolInputDeltaEvent
  | SSEToolResultEvent;

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
