import {
    _decorator, Component, Node, Sprite, SpriteFrame,
    resources, view, math,
} from 'cc';
import { GameConfig } from './GameConfig';
import { WsClient } from './WsClient';

const { ccclass, property } = _decorator;

const API_BASE = 'http://localhost:9080/api/v1';

function getToken(): string {
    try { return localStorage.getItem('token') ?? ''; } catch { return ''; }
}

/**
 * WorldScene
 *
 * 职责：
 *  1. 加载 GameConfig（地图尺寸、出生点）
 *  2. 拉取当前玩家自己的 Role，在地图上创建节点并居中镜头
 *  3. 建立 WebSocket 连接
 *  4. 收到 role_move 事件时：
 *     - 首次出现的 roleId → 按需创建节点 + 按需加载精灵
 *     - 已有节点 → 直接更新位置
 *
 * 玩家不控制 Role 移动；Role 由服务器 tick 驱动，自主活动。
 * 拖拽平移由 WorldMapController 负责，本脚本不干涉。
 */
@ccclass('WorldScene')
export class WorldScene extends Component {

    @property(Node)
    worldMap: Node = null!;

    private _mapPixelW = 2048;
    private _mapPixelH = 2048;

    private _myRoleId = '';

    /**
     * All currently-visible role nodes, keyed by server role ID.
     * Populated on demand as role_move events arrive — no bulk preload.
     */
    private _roleNodes = new Map<string, Node>();

    private _onRoleMove = (payload: unknown) => this._handleRoleMove(payload);

    // ─────────────────────────────────────────────────────────────────────────

    async onLoad() {
        await GameConfig.load();

        const mapCfg = GameConfig.getMap('world');
        if (mapCfg) {
            this._mapPixelW = mapCfg.width_tiles * mapCfg.tile_w;
            this._mapPixelH = mapCfg.height_tiles * mapCfg.tile_h;
        }

        await this._loadMyRole();
        this._connectWs();
    }

    onDestroy() {
        WsClient.instance.off('role_move', this._onRoleMove);
    }

    // ─────────────────────────────────────────────────────────────────────────

    /** 仅加载当前玩家自己的 Role 以确定初始位置和镜头中心点。 */
    private async _loadMyRole() {
        const token = getToken();
        if (!token) return;

        let avatar = '';
        let name   = '';
        let tilePosX = 10;
        let tilePosY = 10;

        try {
            const res = await fetch(`${API_BASE}/agent`, {
                headers: { Authorization: `Bearer ${token}` },
            });
            if (res.ok) {
                const json = await res.json();
                this._myRoleId = json?.data?.id     ?? '';
                avatar         = json?.data?.avatar ?? '';
                name           = json?.data?.name   ?? '';
                tilePosX       = json?.data?.tile_x ?? tilePosX;
                tilePosY       = json?.data?.tile_y ?? tilePosY;
            }
        } catch {
            // 离线 / 开发模式：使用 GameConfig 出生点
        }

        const spawn  = GameConfig.getSpawnPixel('world');
        const pixelX = tilePosX > 0 ? tilePosX * 16 : (spawn?.x ?? 160);
        const pixelY = tilePosY > 0 ? tilePosY * 16 : (spawn?.y ?? 160);

        if (this._myRoleId) {
            const node = this._spawnRoleNode(this._myRoleId, name, avatar, pixelX, pixelY);
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

    // ─────────────────────────────────────────────────────────────────────────

    /**
     * 处理服务器推送的 role_move 事件。
     *
     * 首次收到某个 roleId → 按需创建节点并加载精灵资源。
     * 后续收到同一 roleId → 仅更新位置，不重复加载资源。
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
            // 首次出现：按需创建节点 + 按需加载精灵
            node = this._spawnRoleNode(p.roleId, p.name, p.avatar, pixelX, pixelY);
            this._roleNodes.set(p.roleId, node);
            return; // 位置已在 _spawnRoleNode 里设好
        }

        // 已有节点：只更新位置
        node.setPosition(
            pixelX - this._mapPixelW / 2,
            this._mapPixelH / 2 - pixelY,
            1,
        );
    }

    // ─────────────────────────────────────────────────────────────────────────

    /**
     * 创建一个 Role 节点，挂到 WorldMap 下，设好初始位置。
     * 如果 avatar 非空，异步加载精灵帧（不阻塞主流程）。
     */
    private _spawnRoleNode(
        roleId: string,
        _name: string,
        avatar: string,
        mapPx: number,
        mapPy: number,
    ): Node {
        const node = new Node(roleId);
        node.addComponent(Sprite);
        this.worldMap.addChild(node);

        node.setPosition(
            mapPx - this._mapPixelW / 2,
            this._mapPixelH / 2 - mapPy,
            1,
        );

        if (avatar) {
            const path = `sprites/characters/roles/${avatar}`;
            resources.load(path, SpriteFrame, (err, sf) => {
                if (err || !sf) {
                    console.warn(`[WorldScene] sprite not found: ${path}`);
                    return;
                }
                node.getComponent(Sprite)!.spriteFrame = sf;
            });
        }

        return node;
    }

    private _centerOn(mapPx: number, mapPy: number) {
        const vs      = view.getVisibleSize();
        const halfMapW = this._mapPixelW / 2;
        const halfMapH = this._mapPixelH / 2;

        this.worldMap.setPosition(
            math.clamp(halfMapW - mapPx, vs.width  / 2 - halfMapW, halfMapW - vs.width  / 2),
            math.clamp(mapPy - halfMapH, vs.height / 2 - halfMapH, halfMapH - vs.height / 2),
            0,
        );
    }
}
