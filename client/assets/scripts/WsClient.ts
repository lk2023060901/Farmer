/**
 * WsClient — singleton WebSocket manager for 农趣村.
 *
 * Implemented as a plain TypeScript class (NOT a Cocos Component) so that the
 * connection persists across scene loads.  Import and use via WsClient.instance.
 *
 * Usage:
 *   WsClient.instance.connect(token);
 *   WsClient.instance.on('role_move', payload => { ... });
 *   WsClient.instance.off('role_move', handler);
 */

const WS_BASE = 'ws://localhost:9080/api/v1/ws';

const PING_INTERVAL_MS = 30_000; // send ping every 30 s (server drops after 60 s)

/** Back-off delays for successive reconnect attempts (ms). */
const RECONNECT_DELAYS = [1_000, 2_000, 4_000, 8_000, 16_000];

export type WsEventType =
    | 'farm_update'
    | 'social_event'
    | 'trade_notify'
    | 'notification'
    | 'system_announce'
    | 'pong'
    | 'role_move';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WsHandler = (payload: any) => void;

export class WsClient {

    private static _instance: WsClient | null = null;

    /** Returns the shared singleton. */
    static get instance(): WsClient {
        if (!WsClient._instance) {
            WsClient._instance = new WsClient();
        }
        return WsClient._instance;
    }

    // ─── state ───────────────────────────────────────────────────────────────

    private _ws: WebSocket | null = null;
    private _token = '';
    private _connected = false;
    private _reconnectAttempt = 0;
    private _pingTimer: ReturnType<typeof setInterval> | null = null;
    private _reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    private _handlers = new Map<WsEventType, Set<WsHandler>>();

    // ─── public API ──────────────────────────────────────────────────────────

    /**
     * Open the WebSocket connection authenticated with the given JWT.
     * Safe to call multiple times — reconnects if already closed.
     */
    connect(token: string): void {
        this._token = token;
        this._reconnectAttempt = 0;
        this._doConnect();
    }

    /** Close the connection permanently (no automatic reconnect). */
    disconnect(): void {
        this._clearReconnect();
        this._clearPing();
        if (this._ws) {
            this._ws.onclose = null; // suppress reconnect triggered by close()
            this._ws.close(1000);
            this._ws = null;
        }
        this._connected = false;
    }

    /** Register a handler for a specific event type. */
    on(type: WsEventType, handler: WsHandler): void {
        if (!this._handlers.has(type)) {
            this._handlers.set(type, new Set());
        }
        this._handlers.get(type)!.add(handler);
    }

    /** Remove a previously registered handler. */
    off(type: WsEventType, handler: WsHandler): void {
        this._handlers.get(type)?.delete(handler);
    }

    get isConnected(): boolean { return this._connected; }

    /** 向服务器发送一条 JSON 消息。连接未就绪时静默丢弃。 */
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    send(data: any): void {
        if (this._ws?.readyState === WebSocket.OPEN) {
            this._ws.send(JSON.stringify(data));
        }
    }

    // ─── internals ───────────────────────────────────────────────────────────

    private _doConnect(): void {
        if (!this._token) return;

        const url = `${WS_BASE}?token=${encodeURIComponent(this._token)}`;
        const socket = new WebSocket(url);
        this._ws = socket;

        socket.onopen = () => {
            console.log('[WsClient] connected');
            this._connected = true;
            this._reconnectAttempt = 0;
            this._startPing();
        };

        socket.onmessage = (evt: MessageEvent) => {
            try {
                const msg = JSON.parse(evt.data as string) as { type: WsEventType; payload: unknown };
                this._handlers.get(msg.type)?.forEach(h => h(msg.payload));
            } catch {
                console.warn('[WsClient] failed to parse message:', evt.data);
            }
        };

        socket.onerror = () => {
            console.warn('[WsClient] connection error');
        };

        socket.onclose = (evt: CloseEvent) => {
            console.log(`[WsClient] closed (code=${evt.code})`);
            this._connected = false;
            this._clearPing();
            // Only reconnect on abnormal closure.
            if (evt.code !== 1000) {
                this._scheduleReconnect();
            }
        };
    }

    private _startPing(): void {
        this._pingTimer = setInterval(() => {
            if (this._ws?.readyState === WebSocket.OPEN) {
                this._ws.send(JSON.stringify({ type: 'ping' }));
            }
        }, PING_INTERVAL_MS);
    }

    private _clearPing(): void {
        if (this._pingTimer !== null) {
            clearInterval(this._pingTimer);
            this._pingTimer = null;
        }
    }

    private _scheduleReconnect(): void {
        const delay = RECONNECT_DELAYS[
            Math.min(this._reconnectAttempt, RECONNECT_DELAYS.length - 1)
        ];
        this._reconnectAttempt++;
        console.log(`[WsClient] reconnect in ${delay} ms (attempt ${this._reconnectAttempt})`);
        this._reconnectTimer = setTimeout(() => this._doConnect(), delay);
    }

    private _clearReconnect(): void {
        if (this._reconnectTimer !== null) {
            clearTimeout(this._reconnectTimer);
            this._reconnectTimer = null;
        }
    }
}
