import type { WebsocketEvent, WebsocketEventType } from './generated';

const WEBSOCKET_EVENT_TYPES: Record<WebsocketEventType, true> = {
  'project.created': true,
  'project.deleted': true,
  'card.created': true,
  'card.branch.updated': true,
  'card.moved': true,
  'card.commented': true,
  'card.updated': true,
  'card.todo.added': true,
  'card.todo.updated': true,
  'card.todo.deleted': true,
  'card.acceptance.added': true,
  'card.acceptance.updated': true,
  'card.acceptance.deleted': true,
  'card.deleted_soft': true,
  'card.deleted_hard': true,
  'resync.required': true,
};

export type WebSocketEventContext = {
  selectedProjectSlug: string | null;
  cardDetailsProjectSlug: string | null;
  cardDetailsNumber: number | null;
  loadProjects: () => Promise<void>;
  loadCards: () => Promise<void>;
  retryCardDetails: () => Promise<void>;
};

export function parseWebSocketEvent(raw: string): WebsocketEvent {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new Error('invalid websocket payload JSON');
  }
  if (!parsed || typeof parsed !== 'object') {
    throw new Error('invalid websocket payload shape');
  }

  const payload = parsed as Record<string, unknown>;
  if (typeof payload.type !== 'string' || !isWebSocketEventType(payload.type)) {
    throw new Error(`unknown websocket event type: ${String(payload.type)}`);
  }
  if (typeof payload.project !== 'string') {
    throw new Error('invalid websocket payload project');
  }
  if (typeof payload.timestamp !== 'string') {
    throw new Error('invalid websocket payload timestamp');
  }
  if (payload.card_id !== undefined && typeof payload.card_id !== 'string') {
    throw new Error('invalid websocket payload card_id');
  }
  if (payload.card_number !== undefined && typeof payload.card_number !== 'number') {
    throw new Error('invalid websocket payload card_number');
  }

  return {
    type: payload.type,
    project: payload.project,
    timestamp: payload.timestamp,
    card_id: payload.card_id,
    card_number: payload.card_number,
  };
}

export async function handleWebSocketEvent(payload: WebsocketEvent, context: WebSocketEventContext): Promise<void> {
  switch (payload.type) {
    case 'project.created':
    case 'project.deleted':
      await context.loadProjects();
      return;
    case 'card.created':
    case 'card.branch.updated':
    case 'card.moved':
    case 'card.commented':
    case 'card.updated':
    case 'card.todo.added':
    case 'card.todo.updated':
    case 'card.todo.deleted':
    case 'card.acceptance.added':
    case 'card.acceptance.updated':
    case 'card.acceptance.deleted':
    case 'card.deleted_soft':
    case 'card.deleted_hard':
    case 'resync.required':
      if (!context.selectedProjectSlug || payload.project !== context.selectedProjectSlug) {
        return;
      }
      await context.loadCards();
      if (context.cardDetailsProjectSlug === context.selectedProjectSlug && context.cardDetailsNumber !== null) {
        await context.retryCardDetails();
      }
      return;
  }

  assertNever(payload.type);
}

function isWebSocketEventType(value: string): value is WebsocketEventType {
  return value in WEBSOCKET_EVENT_TYPES;
}

function assertNever(value: never): never {
  throw new Error(`unhandled websocket event type: ${String(value)}`);
}
