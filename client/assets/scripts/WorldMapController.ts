import {
    _decorator, Component, Node, Sprite, SpriteFrame,
    assetManager, ImageAsset, view, EventTouch, EventMouse,
    Graphics, Color, Rect, Size, Texture2D, gfx,
} from 'cc';
import { GameConfig } from './GameConfig';
import { WsClient } from './WsClient';

const { ccclass, property } = _decorator;

const API_BASE    = 'http://localhost:9080/api/v1';
const SPRITE_BASE = 'http://localhost:9080/sprites';

function getToken(): string {
    try { return localStorage.getItem('token') ?? ''; } catch { return ''; }
}

/**
 * WorldMapController
 *
 * 挂在 WorldMap 节点（含 TiledMap 组件）上，负责：
 *  1. 拖拽平移地图（touch + mouse）
 *  2. 加载玩家自己的 Role 并居中镜头
 *  3. 连接 WebSocket，按需渲染其他 Role 节点
 */
@ccclass('WorldMapController')
export class WorldMapController extends Component {

    @property
    mapWidth: number = 2048;

    @property
    mapHeight: number = 2048;

    // ── 拖拽状态 ──────────────────────────────────────────────────────────────

    private _dragging = false;
    private _lastX = 0;
    private _lastY = 0;
    private _dragDist = 0; // 累计拖拽距离，用于区分点击和拖拽

    // ── Role 渲染状态 ─────────────────────────────────────────────────────────

    private _myRoleId = '';
    /** 当前已渲染的 Role 节点，key = 服务器 role ID */
    private _roleNodes = new Map<string, Node>();
    /**
     * 每个 Role 的动画状态：
     *   dirFrames[0]=东, [1]=北, [2]=西, [3]=南，每组6帧（精灵实际顺序）
     *   dirIdx 表示当前朝向
     */
    private _roleStates = new Map<string, {
        sp: Sprite;
        dirFrames: SpriteFrame[][];
        dirIdx: number;
        tileX: number;
        tileY: number;
    }>();

    private _onRoleMove = (p: unknown) => this._handleRoleMove(p);

    // ─────────────────────────────────────────────────────────────────────────

    async onLoad() {
        this._registerInputEvents();

        // 只在 WorldMap 节点上执行世界逻辑
        // （Canvas 上也挂了 WorldMapController，通过节点名称区分）
        console.log(`[WorldMap] onLoad node="${this.node.name}"`);
        if (this.node.name !== 'WorldMap') return;
        console.log('[WorldMap] WorldMap node confirmed, initialising world…');

        await GameConfig.load();
        const mapCfg = GameConfig.getMap('world');
        if (mapCfg) {
            this.mapWidth  = mapCfg.width_tiles * mapCfg.tile_w;
            this.mapHeight = mapCfg.height_tiles * mapCfg.tile_h;
        }
        console.log(`[WorldMap] map size: ${this.mapWidth}×${this.mapHeight}`);

        await this._loadMyRole();
        this._connectWs();
        console.log('[WorldMap] init complete');
    }

    onDestroy() {
        this._unregisterInputEvents();
        WsClient.instance.off('role_move', this._onRoleMove);
    }

    // ── 世界初始化 ────────────────────────────────────────────────────────────

    private async _loadMyRole() {
        const token = getToken();
        if (!token) {
            console.warn('[WorldMap] no token — skipping role load');
            return;
        }

        let avatar = '', tilePosX = 10, tilePosY = 10;

        try {
            const res = await fetch(`${API_BASE}/agent`, {
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                const json = await res.json();
                this._myRoleId = json?.data?.id     ?? '';
                avatar         = json?.data?.avatar ?? '';
                tilePosX       = json?.data?.tile_x ?? tilePosX;
                tilePosY       = json?.data?.tile_y ?? tilePosY;
            }
        } catch { /* 离线/开发模式：使用默认出生点 */ }

        const spawn  = GameConfig.getSpawnPixel('world');
        const pixelX = tilePosX > 0 ? tilePosX * 16 : (spawn?.x ?? 160);
        const pixelY = tilePosY > 0 ? tilePosY * 16 : (spawn?.y ?? 160);

        console.log(`[WorldMap] myRoleId=${this._myRoleId} avatar=${avatar} px=(${pixelX},${pixelY})`);
        if (this._myRoleId) {
            const node = this._spawnRoleNode(this._myRoleId, avatar, pixelX, pixelY, tilePosX, tilePosY);
            this._roleNodes.set(this._myRoleId, node);
        }
        this._centerOn(pixelX, pixelY);
    }

    private _connectWs() {
        const token = getToken();
        if (!token) return;
        const ws = WsClient.instance;
        if (!ws.isConnected) ws.connect(token);
        ws.on('role_move', this._onRoleMove);
    }

    // ── Role 节点管理 ─────────────────────────────────────────────────────────

    /**
     * 首次收到某 roleId 时创建节点+按需加载精灵；
     * 后续只更新坐标。
     */
    private _handleRoleMove(payload: unknown) {
        const p = payload as {
            roleId: string; name: string; avatar: string;
            mapId: string; tileX: number; tileY: number;
        };
        if (!p || p.mapId !== 'world') return;

        const pixelX = p.tileX * 16;
        const pixelY = p.tileY * 16;

        let node = this._roleNodes.get(p.roleId);
        if (!node) {
            node = this._spawnRoleNode(p.roleId, p.avatar, pixelX, pixelY, p.tileX, p.tileY);
            this._roleNodes.set(p.roleId, node);
            return;
        }

        const st = this._roleStates.get(p.roleId);
        if (st) {
            const dx = p.tileX - st.tileX;
            const dy = p.tileY - st.tileY;
            if (dx !== 0 || dy !== 0) {
                // 精灵实际顺序: 0=东 1=北 2=西 3=南
                if (Math.abs(dx) >= Math.abs(dy)) {
                    st.dirIdx = dx > 0 ? 0 : 2;  // 向右→东, 向左→西
                } else {
                    st.dirIdx = dy > 0 ? 3 : 1;  // tileY增→南, tileY减→北
                }
            }
            st.tileX = p.tileX;
            st.tileY = p.tileY;
        }

        node.setPosition(
            pixelX - this.mapWidth  / 2,
            this.mapHeight / 2 - pixelY,
            1,
        );
    }

    /**
     * 创建 Role Sprite 节点，挂到 WorldMap（this.node）下。
     * 如果 avatar 非空，异步加载精灵帧。
     */
    private _spawnRoleNode(roleId: string, avatar: string, mapPx: number, mapPy: number, tileX = 0, tileY = 0): Node {
        const node = new Node(roleId);

        // 主节点只挂 Sprite，避免与 Graphics 冲突（CC3.x 一个节点只能有一个渲染组件）
        const sp = node.addComponent(Sprite);
        sp.sizeMode = Sprite.SizeMode.RAW;

        // 占位红圆放在子节点，精灵加载成功后隐藏
        const dot = new Node('dot');
        const g = dot.addComponent(Graphics);
        g.fillColor = new Color(220, 50, 50, 255);
        g.circle(0, 0, 8);
        g.fill();
        node.addChild(dot);

        this.node.addChild(node);
        node.setPosition(
            mapPx - this.mapWidth  / 2,
            this.mapHeight / 2 - mapPy,
            1,
        );
        node.setScale(4, 4, 1);

        console.log(`[WorldMap] spawnRole id=${roleId} avatar=${avatar} pos=(${mapPx},${mapPy})`);

        // 初始化方向状态（朝南，dirIdx=3）
        const state = { sp, dirFrames: [] as SpriteFrame[][], dirIdx: 3, tileX, tileY };
        this._roleStates.set(roleId, state);

        if (avatar) {
            // 优先使用 idle 动画版（Adam_idle_16x16 → Adam_idle_anim_16x16）
            const animAvatar = avatar.replace('_idle_', '_idle_anim_');
            const url = `${SPRITE_BASE}/${animAvatar}.png`;
            assetManager.loadRemote<ImageAsset>(url, { ext: '.png' }, (err, imgAsset) => {
                if (err || !imgAsset) {
                    console.warn(`[WorldMap] sprite loadRemote failed: ${url}`, err);
                    return;
                }
                const tex = new Texture2D();
                tex.image = imgAsset;
                tex.setFilters(gfx.Filter.POINT, gfx.Filter.POINT);

                // 构建 4方向 × N帧 的帧组（精灵实际顺序: 0=东 1=北 2=西 3=南）
                const FRAME_W      = 16;
                const FRAME_H      = imgAsset.height;
                const totalFrames  = Math.floor(imgAsset.width / FRAME_W);
                const framesPerDir = Math.floor(totalFrames / 4);

                for (let d = 0; d < 4; d++) {
                    const dir: SpriteFrame[] = [];
                    for (let i = 0; i < framesPerDir; i++) {
                        const f = new SpriteFrame();
                        f.texture = tex;
                        f.rect = new Rect((d * framesPerDir + i) * FRAME_W, 0, FRAME_W, FRAME_H);
                        f.originalSize = new Size(FRAME_W, FRAME_H);
                        dir.push(f);
                    }
                    state.dirFrames.push(dir);
                }

                dot.active = false;
                // 保底：如果帧数不够4组，回退到第0组
                const initDir = state.dirFrames.length >= 4 ? 3 : 0;
                state.dirIdx = initDir;
                sp.spriteFrame = state.dirFrames[initDir][0];

                // 以 4 FPS 循环，读取当前方向帧组
                let animIdx = 0;
                this.schedule(() => {
                    if (!sp.isValid) return;
                    const dir = state.dirFrames[state.dirIdx];
                    if (!dir || dir.length === 0) return;
                    animIdx = (animIdx + 1) % dir.length;
                    sp.spriteFrame = dir[animIdx];
                }, 0.25);
                console.log(`[WorldMap] sprite loaded: ${url} totalFrames=${totalFrames} framesPerDir=${framesPerDir}`);
            });
        }

        return node;
    }

    /** 调整 WorldMap 位置，使地图坐标 (mapPx, mapPy) 出现在屏幕中心。*/
    private _centerOn(mapPx: number, mapPy: number) {
        const vs       = view.getVisibleSize();
        const halfMapW = this.mapWidth  / 2;
        const halfMapH = this.mapHeight / 2;

        const nx = Math.max(vs.width  / 2 - halfMapW, Math.min(halfMapW - vs.width  / 2, halfMapW - mapPx));
        const ny = Math.max(vs.height / 2 - halfMapH, Math.min(halfMapH - vs.height / 2, mapPy - halfMapH));
        this.node.setPosition(nx, ny, 0);
        console.log(`[WorldMap] centerOn(${mapPx},${mapPy}) → worldMap pos (${nx.toFixed(0)},${ny.toFixed(0)})`);
    }

    // ── 拖拽平移 ──────────────────────────────────────────────────────────────

    private _registerInputEvents() {
        this.node.on(Node.EventType.TOUCH_START,  this._onTouchStart,  this);
        this.node.on(Node.EventType.TOUCH_MOVE,   this._onTouchMove,   this);
        this.node.on(Node.EventType.TOUCH_END,    this._onTouchEnd,   this);
        this.node.on(Node.EventType.TOUCH_CANCEL, this._onTouchCancel, this);
        this.node.on(Node.EventType.MOUSE_DOWN,   this._onMouseDown,   this);
        this.node.on(Node.EventType.MOUSE_MOVE,   this._onMouseMove,   this);
        this.node.on(Node.EventType.MOUSE_UP,     this._onMouseUp,     this);
    }

    private _unregisterInputEvents() {
        this.node.off(Node.EventType.TOUCH_START,  this._onTouchStart,  this);
        this.node.off(Node.EventType.TOUCH_MOVE,   this._onTouchMove,   this);
        this.node.off(Node.EventType.TOUCH_END,    this._onTouchEnd,   this);
        this.node.off(Node.EventType.TOUCH_CANCEL, this._onTouchCancel, this);
        this.node.off(Node.EventType.MOUSE_DOWN,   this._onMouseDown,   this);
        this.node.off(Node.EventType.MOUSE_MOVE,   this._onMouseMove,   this);
        this.node.off(Node.EventType.MOUSE_UP,     this._onMouseUp,     this);
    }

    private _onTouchStart(e: EventTouch)  { this._startDrag(e.getLocationX(), e.getLocationY()); }
    private _onTouchMove(e: EventTouch)   { this._moveDrag(e.getLocationX(),  e.getLocationY()); }
    private _onTouchEnd(e: EventTouch)    {
        if (this._dragDist < 8) this._onTap(e.getLocationX(), e.getLocationY());
        this._dragging = false;
    }
    private _onTouchCancel()              { this._dragging = false; }

    private _onMouseDown(e: EventMouse) {
        if (e.getButton() === EventMouse.BUTTON_LEFT)
            this._startDrag(e.getLocationX(), e.getLocationY());
    }
    private _onMouseMove(e: EventMouse) { this._moveDrag(e.getLocationX(), e.getLocationY()); }
    private _onMouseUp(e: EventMouse) {
        if (this._dragDist < 8) this._onTap(e.getLocationX(), e.getLocationY());
        this._dragging = false;
    }

    private _startDrag(x: number, y: number) {
        this._dragging = true;
        this._dragDist = 0;
        this._lastX = x;
        this._lastY = y;
    }

    private _moveDrag(x: number, y: number) {
        if (!this._dragging) return;
        const dx = x - this._lastX;
        const dy = y - this._lastY;
        this._dragDist += Math.abs(dx) + Math.abs(dy);
        this._pan(dx, dy);
        this._lastX = x;
        this._lastY = y;
    }

    /** 点击（非拖拽）地图时，向服务器发送 move_to 指令 */
    private _onTap(screenX: number, screenY: number) {
        const vs      = view.getVisibleSize();
        const pos     = this.node.position;
        // 屏幕坐标（左下原点）→ Canvas 世界坐标（居中原点）
        const worldX  = screenX - vs.width  / 2;
        const worldY  = screenY - vs.height / 2;
        // 转为 WorldMap 节点本地坐标
        const localX  = worldX - pos.x;
        const localY  = worldY - pos.y;
        // 转为地图像素坐标（WorldMap 节点原点在地图中心）
        const mapPx   = localX + this.mapWidth  / 2;
        const mapPy   = this.mapHeight / 2 - localY;
        // 转为瓦片坐标
        const tileX   = Math.floor(mapPx / 16);
        const tileY   = Math.floor(mapPy / 16);

        // 点击时立刻更新自己角色的朝向
        const st = this._myRoleId ? this._roleStates.get(this._myRoleId) : null;
        if (st) {
            const dx = tileX - st.tileX;
            const dy = tileY - st.tileY;
            // 精灵实际顺序: 0=东 1=北 2=西 3=南
            if (Math.abs(dx) >= Math.abs(dy)) {
                st.dirIdx = dx > 0 ? 0 : 2;
            } else {
                st.dirIdx = dy > 0 ? 3 : 1;
            }
        }

        const ws = WsClient.instance;
        if (ws.isConnected) {
            ws.send({ type: 'move_to', tileX, tileY });
            console.log(`[WorldMap] move_to tile=(${tileX},${tileY})`);
        }
    }

    private _pan(dx: number, dy: number) {
        const vs       = view.getVisibleSize();
        const halfMapW = this.mapWidth  / 2;
        const halfMapH = this.mapHeight / 2;
        const pos      = this.node.position;

        let nx = pos.x + dx;
        let ny = pos.y + dy;

        if (this.mapWidth  > vs.width)
            nx = Math.max(vs.width  / 2 - halfMapW, Math.min(halfMapW - vs.width  / 2, nx));
        if (this.mapHeight > vs.height)
            ny = Math.max(vs.height / 2 - halfMapH, Math.min(halfMapH - vs.height / 2, ny));

        this.node.setPosition(nx, ny, pos.z);
    }
}
