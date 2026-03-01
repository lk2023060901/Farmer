/**
 * 客户端 Game Config 类型定义
 * 只包含客户端 UI / 逻辑实际需要的字段。
 * 服务器专属字段（strategy_* 等）不在此处定义。
 */

// ── items.json ────────────────────────────────────────────────────────────────

export interface ItemConfig {
    id:           string;
    name:         string;
    category:     string;      // seed / produce / processed / tool / material / gift / special
    sub_category: string;
    icon:         string;      // sprite 资源名
    description:  string;
    stack_limit:  number;
    tradeable:    boolean;
    sellable:     boolean;
    sell_price:   number;
    buy_price:    number;
}

// ── maps.json ─────────────────────────────────────────────────────────────────

export interface SpawnPoint {
    tile_x: number;
    tile_y: number;
}

export interface MapConfig {
    id:           string;   // "world"
    name:         string;   // 显示名
    tmx:          string;   // TiledMap 资源路径（相对 assets/）
    tile_w:       number;   // 单个 tile 宽度（像素）
    tile_h:       number;   // 单个 tile 高度（像素）
    width_tiles:  number;   // 地图总宽（tile 数）
    height_tiles: number;   // 地图总高（tile 数）
    spawn:        SpawnPoint; // 玩家出生 tile 坐标
}

// ── role_archetypes.json ──────────────────────────────────────────────────────
// 客户端只需要展示信息（头像、名称、描述）。
// 人格 / 策略数值是服务器 AI 逻辑，客户端无需关心。

export interface ArchetypeConfig {
    id:          number;
    role_type:   string;   // farmer / merchant / explorer / craftsman
    name:        string;
    description: string;
    avatar:      string;      // idle 静态帧名
    avatar_idle: string;      // idle 动画帧名
    avatar_run:  string;      // run 动画帧名
}
