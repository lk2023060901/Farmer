import { resources, JsonAsset } from 'cc';
import { ItemConfig, ArchetypeConfig, MapConfig } from './types/gameConfig';

/**
 * GameConfig — 游戏静态配置单例
 *
 * 在 World 场景初始化时调用 GameConfig.load()，之后任意模块通过
 * GameConfig.getItem() / GameConfig.getArchetype() / GameConfig.getMap() 同步获取数据。
 */
export class GameConfig {
    private static _items      = new Map<string, ItemConfig>();
    private static _archetypes = new Map<number, ArchetypeConfig>();
    private static _maps       = new Map<string, MapConfig>();
    private static _loaded     = false;

    /** 加载所有配置，应在游戏启动时调用一次 */
    static async load(): Promise<void> {
        if (this._loaded) return;
        await Promise.all([
            this._loadItems(),
            this._loadArchetypes(),
            this._loadMaps(),
        ]);
        this._loaded = true;
        console.log('[GameConfig] loaded', this._items.size, 'items,', this._archetypes.size, 'archetypes,', this._maps.size, 'maps');
    }

    static getItem(id: string): ItemConfig | undefined {
        return this._items.get(id);
    }

    static getArchetype(id: number): ArchetypeConfig | undefined {
        return this._archetypes.get(id);
    }

    static getMap(id: string): MapConfig | undefined {
        return this._maps.get(id);
    }

    /**
     * 返回指定地图的玩家出生像素坐标（Cocos 世界坐标）。
     * tile 坐标 × 单格像素尺寸，原点在地图左上角。
     */
    static getSpawnPixel(mapId: string): { x: number; y: number } | undefined {
        const map = this._maps.get(mapId);
        if (!map) return undefined;
        return {
            x: map.spawn.tile_x * map.tile_w,
            y: map.spawn.tile_y * map.tile_h,
        };
    }

    static getAllItems(): ItemConfig[] {
        return [...this._items.values()];
    }

    static getAllArchetypes(): ArchetypeConfig[] {
        return [...this._archetypes.values()];
    }

    static isLoaded(): boolean { return this._loaded; }

    // ── private ───────────────────────────────────────────────────────────────

    private static _loadItems(): Promise<void> {
        return new Promise((resolve, reject) => {
            resources.load('configs/items', JsonAsset, (err, asset) => {
                if (err) { reject(err); return; }
                const data = asset.json as { items: ItemConfig[] };
                for (const item of data.items) {
                    this._items.set(item.id, item);
                }
                resolve();
            });
        });
    }

    private static _loadArchetypes(): Promise<void> {
        return new Promise((resolve, reject) => {
            resources.load('configs/roles', JsonAsset, (err, asset) => {
                if (err) { reject(err); return; }
                const data = asset.json as { roles: ArchetypeConfig[] };
                for (const arch of data.roles) {
                    this._archetypes.set(arch.id, arch);
                }
                resolve();
            });
        });
    }

    private static _loadMaps(): Promise<void> {
        return new Promise((resolve, reject) => {
            resources.load('configs/maps', JsonAsset, (err, asset) => {
                if (err) { reject(err); return; }
                const data = asset.json as { maps: MapConfig[] };
                for (const map of data.maps) {
                    this._maps.set(map.id, map);
                }
                resolve();
            });
        });
    }
}
