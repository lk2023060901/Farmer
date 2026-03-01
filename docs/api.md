# 农趣村 — API 接口文档

> **文档版本**: v1.0
> **最后更新**: 2026-02-28
> **状态**: 设计阶段（对应 T-001 实现）

---

## 目录

1. [规范约定](#1-规范约定)
2. [数据类型定义](#2-数据类型定义)
3. [认证 Auth](#3-认证-auth)
4. [用户 User](#4-用户-user)
5. [农场 Farm](#5-农场-farm)
6. [建筑 Building](#6-建筑-building)
7. [仓库 Inventory](#7-仓库-inventory)
8. [AI Agent](#8-ai-agent)
9. [社交 Social](#9-社交-social)
10. [村庄 Village](#10-村庄-village)
11. [交易 Trade](#11-交易-trade)
12. [好友 Friend](#12-好友-friend)
13. [动态 Feed](#13-动态-feed)
14. [通知 Notification](#14-通知-notification)
15. [每日签到 Daily](#15-每日签到-daily)
16. [商城 Shop](#16-商城-shop)
17. [支付 Payment](#17-支付-payment)
18. [排行榜 Ranking](#18-排行榜-ranking)
19. [赛季 Season](#19-赛季-season)
20. [WebSocket 事件](#20-websocket-事件)

---

## 1. 规范约定

### 1.1 Base URL

```
生产环境: https://api.nongqucun.com/api
测试环境: https://api-test.nongqucun.com/api
本地开发: http://localhost:8080/api
```

### 1.2 认证方式

所有需要登录的接口，在请求头中携带 JWT Token：

```http
Authorization: Bearer <token>
```

标注 `🔒 需要认证` 的接口必须携带此 Header，否则返回 `40001`。

### 1.3 统一响应格式

**成功响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": { }
}
```

**失败响应**

```json
{
  "code": 40004,
  "message": "体力不足",
  "data": null
}
```

**分页响应（Offset 模式）**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [ ],
    "total": 100,
    "page": 1,
    "pageSize": 20
  }
}
```

**分页响应（Cursor 模式，用于 Feed 流）**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [ ],
    "nextCursor": "eyJpZCI6MTIzfQ==",
    "hasMore": true
  }
}
```

### 1.4 错误码

| 错误码 | 含义 |
|--------|------|
| `0` | 成功 |
| `40001` | 未登录 / Token 失效 |
| `40002` | 无权限 |
| `40003` | 资源不存在 |
| `40004` | 请求参数错误 |
| `40005` | 体力不足 |
| `40006` | 金币不足 |
| `40007` | 钻石不足 |
| `40008` | 仓库已满 |
| `40009` | 等级不满足解锁条件 |
| `40010` | 地块已有作物 |
| `40011` | 作物未成熟 |
| `40012` | 建筑升级条件不满足 |
| `40013` | 好感度不足 |
| `40014` | 超出每日次数限制 |
| `40015` | 物品数量不足 |
| `40016` | 商品已售出 |
| `40017` | 不能对自己操作 |
| `40018` | 村庄等级不满足 |
| `40019` | 请求过于频繁（限流）|
| `40020` | 赛季未开始 |
| `50001` | 服务器内部错误 |
| `50002` | LLM 服务异常 |
| `50003` | 数据库异常 |

### 1.5 通用请求头

```http
Content-Type: application/json
Accept-Language: zh-CN
X-Platform: miniprogram | h5 | pc      # 客户端平台标识
X-Client-Version: 1.0.0                 # 客户端版本
```

### 1.6 限流策略

| 接口类型 | 限制 |
|----------|------|
| 认证接口 | 10次/分钟/IP |
| 普通读接口 | 120次/分钟/用户 |
| 写操作接口 | 30次/分钟/用户 |
| 支付接口 | 5次/分钟/用户 |

超出限制返回 HTTP 429，Body 中 code 为 `40019`。

### 1.7 时间格式

所有时间字段统一使用 **ISO 8601 UTC** 格式：

```
2026-02-28T10:30:00Z
```

---

## 2. 数据类型定义

### 2.1 作物配置（CropConfig）

```typescript
interface CropConfig {
  id: string;           // "radish" | "cabbage" | ...
  name: string;         // "萝卜"
  category: "vegetable" | "grain" | "fruit" | "flower" | "rare";
  growSeconds: number;  // 生长时长（秒）
  sellPrice: number;    // 金币售价
  unlockLevel: number;  // 解锁等级
  season: string[];     // 可种植季节 ["spring", "summer", "autumn", "winter"]
  inSeasonBonus: number; // 当季产量加成，如 1.5 = +50%
}
```

**作物配置表（静态数据，前端缓存）**

| id | name | growSeconds | sellPrice | unlockLevel |
|----|------|-------------|-----------|-------------|
| `radish` | 萝卜 | 1800 | 10 | 1 |
| `cabbage` | 白菜 | 3600 | 18 | 1 |
| `tomato` | 番茄 | 7200 | 35 | 3 |
| `pumpkin` | 南瓜 | 14400 | 60 | 5 |
| `wheat` | 小麦 | 10800 | 40 | 2 |
| `corn` | 玉米 | 21600 | 80 | 6 |
| `rice` | 水稻 | 28800 | 110 | 8 |
| `strawberry` | 草莓 | 14400 | 55 | 4 |
| `watermelon` | 西瓜 | 43200 | 150 | 10 |
| `apple` | 苹果树 | 86400 | 200 | 15 |
| `sunflower` | 向日葵 | 7200 | 30 | 3 |
| `rose` | 玫瑰 | 21600 | 90 | 7 |
| `golden_pumpkin` | 金色南瓜 | 86400 | 500 | 0 |
| `starlight_flower` | 星光花 | 172800 | 800 | 0 |

### 2.2 地块（Plot）

```typescript
interface Plot {
  x: number;                                         // 列 0-7
  y: number;                                         // 行 0-7
  type: "empty" | "planted" | "building";
  cropId?: string;                                   // 当 type = "planted"
  plantedAt?: string;                                // ISO 8601
  stage?: "seed" | "sprout" | "growing" | "mature"; // 生长阶段
  quality?: "normal" | "good" | "excellent";         // 品质（成熟后确定）
  wateredAt?: string;                                // 最后浇水时间
  fertilized?: boolean;                              // 是否施肥
  harvestableAt?: string;                            // 可收获时间（计算字段）
  growthPercent?: number;                            // 生长进度 0-100
}
```

### 2.3 建筑（Building）

```typescript
interface Building {
  id: string;
  type: "farmland" | "warehouse" | "coop" | "barn" | "pen"
      | "workshop" | "well" | "fence" | "mailbox";
  level: number;       // 1-3
  x: number;           // 农场坐标
  y: number;
  state: "normal" | "building" | "upgrading";
  finishAt?: string;   // 建造/升级完成时间
}
```

### 2.4 物品（Item）

```typescript
interface InventoryItem {
  itemId: string;      // "radish" | "wheat_seed" | "coin" | ...
  itemType: "crop" | "seed" | "animal_product" | "material"
           | "recipe" | "tool" | "special" | "decoration";
  name: string;
  quantity: number;
  icon: string;        // sprite atlas 中的帧名
}
```

### 2.5 人格（Personality）

```typescript
interface Personality {
  extroversion: number;  // 外向程度 1-10（高=主动社交）
  generosity: number;    // 慷慨程度 1-10（高=喜欢赠礼）
  adventure: number;     // 冒险程度 1-10（高=种稀有作物）
}
```

### 2.6 社交关系（Relationship）

```typescript
type RelationLevel = "stranger" | "acquaintance" | "friend" | "close_friend" | "best_friend";

interface Relationship {
  targetUserId: string;
  targetAgentName: string;
  targetAvatar: string;
  affinity: number;        // 好感度 0-100
  level: RelationLevel;
  lastInteractAt: string;
}
```

---

## 3. 认证 Auth

### 3.1 微信小程序登录

```http
POST /auth/wx-login
```

**请求体**

```json
{
  "code": "wx_login_code_from_wx_login_api",
  "nickname": "小明",
  "avatar": "https://..."
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expiresAt": "2026-03-29T10:30:00Z",
    "isNewUser": true,
    "userId": "usr_abc123"
  }
}
```

**说明**

- `code` 来自小程序端 `wx.login()` 返回
- 后端通过 `code2session` 换取 openid，完成静默登录
- `isNewUser = true` 时，前端触发新手引导流程

---

### 3.2 H5 / PC 手机号登录

```http
POST /auth/phone-login
```

**请求体**

```json
{
  "phone": "13800138000",
  "code": "123456"
}
```

**响应**（同 3.1）

---

### 3.3 发送短信验证码

```http
POST /auth/sms-code
```

**请求体**

```json
{
  "phone": "13800138000"
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": { "expiresIn": 300 }
}
```

---

### 3.4 刷新 Token

```http
POST /auth/refresh
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "token": "eyJ...",
    "expiresAt": "2026-03-29T10:30:00Z"
  }
}
```

---

### 3.5 登出

```http
POST /auth/logout
🔒 需要认证
```

**响应**

```json
{ "code": 0, "message": "ok", "data": null }
```

---

## 4. 用户 User

### 4.1 获取当前用户信息

```http
GET /user/profile
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "userId": "usr_abc123",
    "nickname": "小明",
    "avatar": "https://...",
    "level": 5,
    "exp": 2340,
    "expToNext": 3000,
    "coins": 8800,
    "diamonds": 120,
    "stamina": 80,
    "maxStamina": 100,
    "staminaRecoverAt": "2026-02-28T11:00:00Z",
    "friendshipPoints": 350,
    "villageId": "vil_xyz789",
    "villageName": "桃花村",
    "createdAt": "2026-01-01T00:00:00Z"
  }
}
```

---

### 4.2 更新用户信息

```http
PUT /user/profile
🔒 需要认证
```

**请求体**（字段均可选）

```json
{
  "nickname": "小红",
  "avatar": "https://..."
}
```

**响应**（返回更新后的 profile，结构同 4.1）

---

### 4.3 获取指定用户公开信息

```http
GET /user/:userId/public
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "userId": "usr_def456",
    "nickname": "小红",
    "avatar": "https://...",
    "level": 8,
    "villageName": "桃花村",
    "agentName": "小花",
    "agentAvatar": "sprite:agent-001",
    "farmPreview": "https://..."
  }
}
```

---

## 5. 农场 Farm

### 5.1 获取自己的农场

```http
GET /farm
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "farmId": "farm_abc",
    "ownerId": "usr_abc123",
    "name": "小明的农场",
    "level": 3,
    "specialty": ["tomato"],
    "plots": [
      {
        "x": 0, "y": 0,
        "type": "planted",
        "cropId": "radish",
        "plantedAt": "2026-02-28T08:00:00Z",
        "stage": "mature",
        "quality": "good",
        "wateredAt": "2026-02-28T09:00:00Z",
        "fertilized": false,
        "harvestableAt": "2026-02-28T08:30:00Z",
        "growthPercent": 100
      },
      {
        "x": 1, "y": 0,
        "type": "empty"
      }
    ],
    "buildings": [
      {
        "id": "bld_001",
        "type": "farmland",
        "level": 2,
        "x": 0, "y": 0,
        "state": "normal"
      }
    ],
    "lastTickAt": "2026-02-28T10:25:00Z"
  }
}
```

---

### 5.2 拜访他人农场

```http
GET /farm/:userId
🔒 需要认证
```

**说明**

- 返回目标用户农场的**只读**视图
- 部分字段（如 `quality` 品质）仅好友（好感度 ≥ 50）可见

**响应**（结构同 5.1，增加字段）

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "farmId": "farm_def",
    "ownerId": "usr_def456",
    "ownerNickname": "小红",
    "ownerAvatar": "https://...",
    "agentName": "小花",
    "relationship": {
      "affinity": 65,
      "level": "friend"
    },
    "canWaterHelp": true,
    "plots": [ ],
    "buildings": [ ]
  }
}
```

---

### 5.3 种植

```http
POST /farm/plant
🔒 需要认证
```

**请求体**

```json
{
  "plotX": 2,
  "plotY": 1,
  "cropId": "tomato"
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "plot": {
      "x": 2, "y": 1,
      "type": "planted",
      "cropId": "tomato",
      "plantedAt": "2026-02-28T10:30:00Z",
      "stage": "seed",
      "harvestableAt": "2026-02-28T12:30:00Z",
      "growthPercent": 0
    },
    "staminaCost": 2,
    "staminaLeft": 78
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40003` | 地块不存在 |
| `40005` | 体力不足 |
| `40008` | 仓库无此种子 |
| `40009` | 等级不满足解锁 |
| `40010` | 地块已有作物 |

---

### 5.4 收获

```http
POST /farm/harvest
🔒 需要认证
```

**请求体**

```json
{
  "plotX": 2,
  "plotY": 1
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "crop": {
      "cropId": "tomato",
      "quantity": 2,
      "quality": "good",
      "sellPrice": 35
    },
    "rewards": {
      "coins": 0,
      "exp": 10
    },
    "plot": {
      "x": 2, "y": 1,
      "type": "empty"
    },
    "staminaCost": 2,
    "staminaLeft": 76
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40011` | 作物未成熟 |
| `40008` | 仓库已满 |

---

### 5.5 浇水

```http
POST /farm/water
🔒 需要认证
```

**请求体**

```json
{
  "plotX": 2,
  "plotY": 1
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "plot": {
      "x": 2, "y": 1,
      "wateredAt": "2026-02-28T10:35:00Z",
      "growthPercent": 25
    },
    "growthBonus": 0.2,
    "staminaCost": 2,
    "staminaLeft": 74
  }
}
```

---

### 5.6 帮好友浇水

```http
POST /farm/:userId/water-help
🔒 需要认证
```

**请求体**

```json
{
  "plotX": 0,
  "plotY": 0
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "friendshipPointsGained": 3,
    "helpCountToday": 1,
    "maxHelpCountDaily": 3
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40013` | 好感度不足（需 ≥ 20 点头之交）|
| `40014` | 今日帮忙次数已达上限（3次）|
| `40017` | 不能帮助自己 |

---

## 6. 建筑 Building

### 6.1 建造建筑

```http
POST /building/build
🔒 需要认证
```

**请求体**

```json
{
  "type": "coop",
  "x": 4,
  "y": 2
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "building": {
      "id": "bld_002",
      "type": "coop",
      "level": 1,
      "x": 4, "y": 2,
      "state": "building",
      "finishAt": "2026-02-28T12:30:00Z"
    },
    "resourceCost": {
      "coins": 300,
      "wood": 20
    }
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40006` | 金币不足 |
| `40009` | 农场等级不满足 |
| `40015` | 建材不足 |

---

### 6.2 升级建筑

```http
POST /building/:buildingId/upgrade
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "building": {
      "id": "bld_001",
      "type": "farmland",
      "level": 2,
      "state": "upgrading",
      "finishAt": "2026-02-28T14:00:00Z"
    },
    "resourceCost": {
      "coins": 500,
      "wood": 20,
      "stone": 10
    },
    "newCapacity": 8
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40012` | 已是最高等级 / 升级条件不满足 |

---

### 6.3 加速建造（钻石）

```http
POST /building/:buildingId/speedup
🔒 需要认证
```

**请求体**

```json
{
  "diamonds": 20
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "building": {
      "id": "bld_001",
      "state": "normal",
      "level": 2
    },
    "diamondsSpent": 20,
    "diamondsLeft": 100
  }
}
```

---

### 6.4 获取建筑配置列表

```http
GET /building/configs
```

**说明**: 无需认证，静态配置，客户端应缓存。

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "configs": [
      {
        "type": "farmland",
        "name": "农田",
        "levels": [
          {
            "level": 1,
            "capacity": 4,
            "cost": { "coins": 0 },
            "unlockFarmLevel": 1
          },
          {
            "level": 2,
            "capacity": 8,
            "cost": { "coins": 500, "wood": 20, "stone": 10 },
            "unlockFarmLevel": 5
          },
          {
            "level": 3,
            "capacity": 12,
            "cost": { "coins": 1500, "wood": 50, "stone": 30 },
            "unlockFarmLevel": 10
          }
        ]
      }
    ]
  }
}
```

---

## 7. 仓库 Inventory

### 7.1 获取仓库列表

```http
GET /inventory
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 必须 | 说明 |
|------|------|------|------|
| `type` | string | 否 | 按类型筛选：`crop` / `seed` / `animal_product` / `material` / `special` |
| `page` | int | 否 | 默认 1 |
| `pageSize` | int | 否 | 默认 40，最大 100 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "itemId": "tomato",
        "itemType": "crop",
        "name": "番茄",
        "quantity": 12,
        "icon": "plant-004",
        "sellPrice": 35
      },
      {
        "itemId": "wheat_seed",
        "itemType": "seed",
        "name": "小麦种子",
        "quantity": 5,
        "icon": "plant-001",
        "sellPrice": null
      }
    ],
    "total": 28,
    "capacity": 100,
    "used": 28,
    "page": 1,
    "pageSize": 40
  }
}
```

---

### 7.2 出售物品

```http
POST /inventory/sell
🔒 需要认证
```

**请求体**

```json
{
  "itemId": "tomato",
  "quantity": 5
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "coinsEarned": 175,
    "coinsTotal": 9000,
    "itemLeft": 7
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40015` | 物品数量不足 |

---

### 7.3 使用物品

```http
POST /inventory/use
🔒 需要认证
```

**请求体**

```json
{
  "itemId": "stamina_potion",
  "quantity": 1
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "effect": {
      "type": "stamina",
      "value": 50
    },
    "staminaTotal": 100,
    "itemLeft": 2
  }
}
```

---

## 8. AI Agent

### 8.1 获取 Agent 信息

```http
GET /agent
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "agentId": "agent_abc",
    "name": "小助",
    "avatar": "sprite:agent-003",
    "personality": {
      "extroversion": 7,
      "generosity": 5,
      "adventure": 6
    },
    "personalityLabel": "热情外向的探索者",
    "behaviorSummary": "今天已串门 2 次，完成交易 1 笔，自动种植 4 块地",
    "lastActiveAt": "2026-02-28T10:25:00Z"
  }
}
```

---

### 8.2 获取策略偏好

```http
GET /agent/strategy
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "managementStyle": 3,
    "plantingPreference": 2,
    "socialTendency": 4,
    "tradeStrategy": 3,
    "resourceAllocation": 2
  }
}
```

**字段说明**（均为 1-5 整数）

| 字段 | 1 端 | 5 端 |
|------|------|------|
| `managementStyle` | 保守经营 | 激进扩张 |
| `plantingPreference` | 短期高频 | 长期高收益 |
| `socialTendency` | 低调独处 | 热情社交 |
| `tradeStrategy` | 囤货惜售 | 快速周转 |
| `resourceAllocation` | 优先种植 | 优先建造 |

---

### 8.3 更新策略偏好

```http
PUT /agent/strategy
🔒 需要认证
```

**请求体**（字段均可选，只传需要修改的字段）

```json
{
  "socialTendency": 5,
  "plantingPreference": 4
}
```

**响应**（返回完整策略，结构同 8.2）

---

### 8.4 获取 Agent 行为日志

```http
GET /agent/logs
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 必须 | 说明 |
|------|------|------|------|
| `cursor` | string | 否 | 游标分页 |
| `pageSize` | int | 否 | 默认 20 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "id": "log_001",
        "type": "auto_plant",
        "content": "自动种植了 4 块萝卜",
        "icon": "plant",
        "createdAt": "2026-02-28T10:25:00Z"
      },
      {
        "id": "log_002",
        "type": "social_visit",
        "content": "拜访了小红的农场，和小花聊得很愉快，好感度 +5",
        "icon": "social",
        "targetUserId": "usr_def456",
        "targetNickname": "小红",
        "createdAt": "2026-02-28T09:10:00Z"
      }
    ],
    "nextCursor": "eyJpZCI6ImxvZ18wMDIifQ==",
    "hasMore": true
  }
}
```

---

## 9. 社交 Social

### 9.1 获取关系列表

```http
GET /social/relationships
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "targetUserId": "usr_def456",
        "targetAgentName": "小花",
        "targetAvatar": "https://...",
        "targetNickname": "小红",
        "affinity": 65,
        "level": "friend",
        "lastInteractAt": "2026-02-28T09:10:00Z"
      }
    ],
    "total": 8
  }
}
```

---

### 9.2 获取与某人的聊天记录

```http
GET /social/chat/:targetUserId
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 必须 | 说明 |
|------|------|------|------|
| `cursor` | string | 否 | 游标（向前翻页） |
| `pageSize` | int | 否 | 默认 20 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "id": "chat_001",
        "scene": "visit",
        "speaker": "agent_abc",
        "speakerName": "小助",
        "content": "哇，你的番茄长得真好！什么时候教我种啊？",
        "createdAt": "2026-02-28T09:10:00Z"
      },
      {
        "id": "chat_002",
        "scene": "visit",
        "speaker": "agent_def",
        "speakerName": "小花",
        "content": "哈哈谢谢，其实就是多浇水而已，你要不要来点种子？",
        "createdAt": "2026-02-28T09:10:05Z"
      }
    ],
    "relationship": {
      "affinity": 65,
      "level": "friend"
    },
    "prevCursor": "eyJpZCI6ImNoYXRfMDAxIn0=",
    "hasMore": false
  }
}
```

---

### 9.3 赠礼

```http
POST /social/gift
🔒 需要认证
```

**请求体**

```json
{
  "targetUserId": "usr_def456",
  "itemId": "tomato",
  "quantity": 3,
  "message": "送你一些番茄！"
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "affinityGained": 15,
    "affinityTotal": 80,
    "newLevel": "close_friend",
    "staminaCost": 5,
    "staminaLeft": 69
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40013` | 好感度不足（需 ≥ 50）|
| `40014` | 今日赠礼次数已满（2次）|
| `40015` | 物品不足 |
| `40017` | 不能赠礼给自己 |

---

### 9.4 发起求助

```http
POST /social/help-request
🔒 需要认证
```

**请求体**

```json
{
  "targetUserId": "usr_def456",
  "resourceType": "seed",
  "resourceId": "wheat_seed",
  "quantity": 5,
  "message": "我的小麦种子不够了，能给我一些吗？"
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "requestId": "help_001",
    "status": "pending"
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40013` | 好感度不足（需 ≥ 50）|
| `40014` | 今日求助次数已满（1次）|

---

### 9.5 响应求助

```http
POST /social/help-respond
🔒 需要认证
```

**请求体**

```json
{
  "requestId": "help_001",
  "accept": true
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "friendshipPointsGained": 15,
    "affinityGained": 10,
    "itemTransferred": {
      "itemId": "wheat_seed",
      "quantity": 5
    }
  }
}
```

---

## 10. 村庄 Village

### 10.1 获取村庄信息

```http
GET /village
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "villageId": "vil_xyz789",
    "name": "桃花村",
    "level": 2,
    "contribution": 8500,
    "nextLevelContribution": 20000,
    "memberCount": 12,
    "maxMemberCount": 20,
    "specialty": ["tomato", "strawberry"],
    "announcements": [
      {
        "id": "ann_001",
        "content": "欢迎新村民小明加入！",
        "createdAt": "2026-02-28T08:00:00Z"
      }
    ],
    "currentProject": {
      "id": "proj_001",
      "name": "修建村庄集市",
      "progress": 6800,
      "target": 10000,
      "rewards": "解锁村庄集市"
    }
  }
}
```

---

### 10.2 获取村庄成员列表

```http
GET /village/members
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "userId": "usr_abc123",
        "nickname": "小明",
        "avatar": "https://...",
        "level": 5,
        "agentName": "小助",
        "isOnline": true,
        "contribution": 1200,
        "affinity": 65,
        "affinityLevel": "friend",
        "joinedAt": "2026-01-01T00:00:00Z"
      }
    ],
    "total": 12
  }
}
```

---

### 10.3 获取共建任务列表

```http
GET /village/projects
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "active": {
      "id": "proj_001",
      "name": "修建村庄集市",
      "description": "需要全村贡献木材和金币",
      "requirements": [
        { "type": "material", "itemId": "wood", "required": 500, "contributed": 320 },
        { "type": "material", "itemId": "stone", "required": 300, "contributed": 180 },
        { "type": "coins", "required": 10000, "contributed": 6300 }
      ],
      "progress": 68,
      "myContribution": 800,
      "topContributors": [
        { "userId": "usr_abc123", "nickname": "小明", "contribution": 1200 }
      ]
    },
    "completed": [
      {
        "id": "proj_000",
        "name": "修缮村路",
        "completedAt": "2026-02-20T18:00:00Z"
      }
    ]
  }
}
```

---

### 10.4 向共建任务捐献

```http
POST /village/contribute
🔒 需要认证
```

**请求体**

```json
{
  "projectId": "proj_001",
  "contributions": [
    { "type": "material", "itemId": "wood", "quantity": 50 },
    { "type": "coins", "quantity": 500 }
  ]
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "myContribution": 1300,
    "projectProgress": 72,
    "projectCompleted": false,
    "affinityGained": 8
  }
}
```

---

### 10.5 发布村庄公告（仅村长）

```http
POST /village/announcement
🔒 需要认证
```

**请求体**

```json
{
  "content": "明天赶集日，大家多上架商品！"
}
```

**响应**

```json
{ "code": 0, "message": "ok", "data": { "id": "ann_002" } }
```

---

## 11. 交易 Trade

### 11.1 获取市场商品列表

```http
GET /trade/market
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 必须 | 说明 |
|------|------|------|------|
| `scope` | string | 否 | `village`（默认）/ `cross_village` |
| `category` | string | 否 | 物品类型筛选 |
| `keyword` | string | 否 | 关键词搜索 |
| `sortBy` | string | 否 | `price_asc` / `price_desc` / `latest` |
| `page` | int | 否 | 默认 1 |
| `pageSize` | int | 否 | 默认 20 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "orderId": "ord_001",
        "itemId": "tomato",
        "itemName": "番茄",
        "itemIcon": "plant-004",
        "itemType": "crop",
        "quality": "good",
        "quantity": 10,
        "priceEach": 30,
        "totalPrice": 300,
        "sellerId": "usr_def456",
        "sellerNickname": "小红",
        "sellerVillage": "桃花村",
        "isFriend": true,
        "feeRate": 0,
        "listedAt": "2026-02-28T09:00:00Z",
        "expiresAt": "2026-03-07T09:00:00Z"
      }
    ],
    "priceReference": {
      "tomato": { "marketAvg": 33, "lastUpdated": "2026-02-28T09:00:00Z" }
    },
    "total": 48,
    "page": 1,
    "pageSize": 20
  }
}
```

---

### 11.2 上架商品

```http
POST /trade/list
🔒 需要认证
```

**请求体**

```json
{
  "itemId": "tomato",
  "quantity": 10,
  "priceEach": 30
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "orderId": "ord_002",
    "itemId": "tomato",
    "quantity": 10,
    "priceEach": 30,
    "feeRate": 0.05,
    "estimatedIncome": 285,
    "expiresAt": "2026-03-07T10:30:00Z"
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40015` | 物品不足 |
| `40018` | 跨村大集市需村庄 Lv.4 |

---

### 11.3 购买商品

```http
POST /trade/buy
🔒 需要认证
```

**请求体**

```json
{
  "orderId": "ord_001",
  "quantity": 3
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "coinsSpent": 90,
    "coinsLeft": 8910,
    "itemReceived": {
      "itemId": "tomato",
      "quantity": 3
    },
    "affinityGained": 5
  }
}
```

**错误码**

| code | 场景 |
|------|------|
| `40006` | 金币不足 |
| `40008` | 仓库已满 |
| `40016` | 商品已售出 |
| `40017` | 不能购买自己的商品 |

---

### 11.4 下架商品

```http
DELETE /trade/orders/:orderId
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "returnedItem": {
      "itemId": "tomato",
      "quantity": 7
    }
  }
}
```

---

### 11.5 我的交易记录

```http
GET /trade/my-orders
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `role` | string | `seller`（默认）/ `buyer` |
| `status` | string | `active` / `sold` / `expired` / `cancelled` |
| `page` | int | 默认 1 |

**响应**（包含订单列表，结构参考 11.1）

---

## 12. 好友 Friend

### 12.1 好友列表

```http
GET /friend/list
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "userId": "usr_def456",
        "nickname": "小红",
        "avatar": "https://...",
        "level": 8,
        "isOnline": false,
        "lastOnlineAt": "2026-02-28T08:00:00Z",
        "affinity": 65,
        "affinityLevel": "friend",
        "canDelegate": false
      }
    ],
    "total": 5
  }
}
```

---

### 12.2 发送好友申请

```http
POST /friend/request
🔒 需要认证
```

**请求体**

```json
{
  "targetUserId": "usr_ghi789",
  "message": "我是小明，我们在集市交易过，加个好友吧！"
}
```

**响应**

```json
{ "code": 0, "message": "ok", "data": { "requestId": "freq_001" } }
```

---

### 12.3 处理好友申请

```http
POST /friend/request/:requestId/respond
🔒 需要认证
```

**请求体**

```json
{ "accept": true }
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "friendship": {
      "userId": "usr_ghi789",
      "nickname": "小李",
      "affinity": 20,
      "affinityLevel": "acquaintance"
    }
  }
}
```

---

### 12.4 待处理的好友申请

```http
GET /friend/requests
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "received": [
      {
        "requestId": "freq_002",
        "fromUserId": "usr_jkl012",
        "fromNickname": "小李",
        "fromAvatar": "https://...",
        "message": "加个好友！",
        "createdAt": "2026-02-28T10:00:00Z"
      }
    ],
    "sent": [ ]
  }
}
```

---

### 12.5 删除好友

```http
DELETE /friend/:userId
🔒 需要认证
```

**响应**

```json
{ "code": 0, "message": "ok", "data": null }
```

---

### 12.6 搜索玩家

```http
GET /friend/search
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 必须 | 说明 |
|------|------|------|------|
| `userId` | string | 是 | 精确搜索用户 ID |

**响应**（返回用户公开信息，结构同 4.3）

---

## 13. 动态 Feed

### 13.1 获取动态流

```http
GET /feed
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `scope` | string | `village`（默认）/ `friend` |
| `cursor` | string | 游标分页 |
| `pageSize` | int | 默认 20 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "id": "feed_001",
        "type": "social_visit",
        "userId": "usr_def456",
        "nickname": "小红",
        "avatar": "https://...",
        "content": "小花拜访了小助，聊了很久关于种植经验的话题",
        "icon": "social",
        "likeCount": 3,
        "isLiked": false,
        "commentCount": 1,
        "createdAt": "2026-02-28T09:10:00Z"
      },
      {
        "id": "feed_002",
        "type": "harvest_milestone",
        "userId": "usr_abc123",
        "nickname": "小明",
        "content": "收获了第 100 棵番茄！",
        "icon": "harvest",
        "likeCount": 7,
        "isLiked": true,
        "commentCount": 2,
        "createdAt": "2026-02-28T08:45:00Z"
      }
    ],
    "nextCursor": "eyJpZCI6ImZlZWRfMDAyIn0=",
    "hasMore": true
  }
}
```

**动态类型（type）枚举**

| type | 含义 |
|------|------|
| `social_visit` | Agent 串门 |
| `social_gift` | Agent 赠礼 |
| `social_trade` | 完成交易 |
| `harvest_milestone` | 收获里程碑 |
| `building_complete` | 建筑建造完成 |
| `level_up` | 农场升级 |
| `village_contribute` | 共建贡献 |
| `random_event` | 随机事件 |
| `friendship_upgrade` | 关系升级 |

---

### 13.2 点赞动态

```http
POST /feed/:feedId/like
🔒 需要认证
```

**响应**

```json
{ "code": 0, "message": "ok", "data": { "likeCount": 4, "isLiked": true } }
```

---

### 13.3 评论动态

```http
POST /feed/:feedId/comment
🔒 需要认证
```

**请求体**

```json
{ "content": "厉害！" }
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "commentId": "cmt_001",
    "content": "厉害！",
    "createdAt": "2026-02-28T10:35:00Z"
  }
}
```

---

### 13.4 离线摘要

```http
GET /feed/offline-summary
🔒 需要认证
```

**说明**: 用户上线后调用，获取离线期间汇总信息。调用后标记为已读，不重复展示。

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "offlineDuration": 28800,
    "summary": {
      "autoHarvested": { "count": 6, "coinsEarned": 480 },
      "autoPlanted": { "count": 4 },
      "socialVisits": [
        {
          "visitorNickname": "小红",
          "agentName": "小花",
          "summary": "聊了你的番茄种植心得",
          "affinityChange": 5
        }
      ],
      "tradeCompleted": [
        {
          "itemName": "白菜",
          "quantity": 8,
          "coinsEarned": 144
        }
      ],
      "events": [
        {
          "type": "random_weather",
          "content": "昨晚下了一场雨，你的花卉长势更旺了"
        }
      ],
      "totalCoinsEarned": 624,
      "totalExpGained": 85
    }
  }
}
```

---

## 14. 通知 Notification

### 14.1 获取通知列表

```http
GET /notification
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `type` | string | `social` / `trade` / `village` / `system` / `event` |
| `unreadOnly` | bool | 默认 false |
| `page` | int | 默认 1 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "id": "ntf_001",
        "type": "trade",
        "title": "交易成功",
        "content": "你上架的 10 个番茄已售出，获得金币 285",
        "isRead": false,
        "actionType": "goto_trade",
        "actionData": { "orderId": "ord_001" },
        "createdAt": "2026-02-28T10:20:00Z"
      }
    ],
    "unreadCount": 3,
    "total": 28,
    "page": 1,
    "pageSize": 20
  }
}
```

---

### 14.2 标记已读

```http
POST /notification/read
🔒 需要认证
```

**请求体**

```json
{
  "notificationIds": ["ntf_001", "ntf_002"]
}
```

**响应**

```json
{ "code": 0, "message": "ok", "data": { "unreadCount": 1 } }
```

---

### 14.3 全部标为已读

```http
POST /notification/read-all
🔒 需要认证
```

**响应**

```json
{ "code": 0, "message": "ok", "data": { "unreadCount": 0 } }
```

---

## 15. 每日签到 Daily

### 15.1 获取签到状态

```http
GET /daily/status
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "todayChecked": false,
    "consecutiveDays": 6,
    "todayReward": {
      "day": 6,
      "type": "rare_seed",
      "quantity": 1,
      "description": "稀有种子 ×1"
    },
    "weekCalendar": [
      { "day": 1, "checked": true,  "reward": "金币 ×100" },
      { "day": 2, "checked": true,  "reward": "种子 ×5" },
      { "day": 3, "checked": true,  "reward": "体力药水 ×1" },
      { "day": 4, "checked": true,  "reward": "金币 ×200" },
      { "day": 5, "checked": true,  "reward": "友谊点 ×50" },
      { "day": 6, "checked": false, "reward": "稀有种子 ×1" },
      { "day": 7, "checked": false, "reward": "钻石 ×10 + 专属装饰" }
    ]
  }
}
```

---

### 15.2 签到

```http
POST /daily/checkin
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "day": 6,
    "consecutiveDays": 6,
    "reward": {
      "type": "rare_seed",
      "quantity": 1,
      "description": "稀有种子 ×1"
    },
    "tomorrow": {
      "day": 7,
      "description": "钻石 ×10 + 专属装饰"
    }
  }
}
```

---

## 16. 商城 Shop

### 16.1 友谊商店

```http
GET /shop/friendship
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "myFriendshipPoints": 350,
    "refreshAt": "2026-03-06T00:00:00Z",
    "items": [
      {
        "id": "fs_001",
        "itemId": "golden_pumpkin_seed",
        "itemName": "金色南瓜种子",
        "itemIcon": "plant-013",
        "cost": 200,
        "stock": 3,
        "stockLeft": 2
      },
      {
        "id": "fs_002",
        "itemId": "deco_sunflower_field",
        "itemName": "向日葵农场装饰",
        "itemIcon": "furn-015",
        "cost": 150,
        "stock": 5,
        "stockLeft": 5
      }
    ]
  }
}
```

---

### 16.2 兑换友谊商品

```http
POST /shop/friendship/exchange
🔒 需要认证
```

**请求体**

```json
{
  "shopItemId": "fs_001",
  "quantity": 1
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "pointsSpent": 200,
    "pointsLeft": 150,
    "itemReceived": {
      "itemId": "golden_pumpkin_seed",
      "quantity": 1
    }
  }
}
```

---

### 16.3 钻石商城

```http
GET /shop/diamond
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `category` | string | `decoration` / `acceleration` / `expansion` / `special` |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "myDiamonds": 120,
    "categories": [
      {
        "id": "decoration",
        "name": "装饰",
        "items": [
          {
            "id": "deco_001",
            "name": "春日樱花主题",
            "description": "农场皮肤",
            "icon": "deco-cherry",
            "price": 68,
            "limitDaily": null,
            "owned": false
          }
        ]
      },
      {
        "id": "acceleration",
        "name": "加速道具",
        "items": [
          {
            "id": "acc_001",
            "name": "体力药水",
            "description": "恢复 50 点体力",
            "icon": "item-potion",
            "price": 5,
            "limitDaily": 3,
            "boughtToday": 1
          }
        ]
      }
    ]
  }
}
```

---

### 16.4 购买钻石商品

```http
POST /shop/diamond/buy
🔒 需要认证
```

**请求体**

```json
{
  "shopItemId": "acc_001",
  "quantity": 1
}
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "diamondsSpent": 5,
    "diamondsLeft": 115,
    "itemReceived": {
      "itemId": "stamina_potion",
      "quantity": 1
    }
  }
}
```

---

## 17. 支付 Payment

### 17.1 创建充值订单

```http
POST /payment/recharge
🔒 需要认证
```

**请求体**

```json
{
  "packageId": "pkg_30yuan",
  "platform": "miniprogram"
}
```

**充值套餐（packageId）**

| packageId | 价格 | 钻石 | 赠送 |
|-----------|------|------|------|
| `pkg_6yuan` | ¥6 | 60 | — |
| `pkg_30yuan` | ¥30 | 330 | +30 |
| `pkg_98yuan` | ¥98 | 1100 | +120 |
| `pkg_198yuan` | ¥198 | 2400 | +400 |
| `pkg_first` | ¥1 | 100 | 首充专属装饰 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "orderId": "pay_order_001",
    "wxPayParams": {
      "timeStamp": "1709123456",
      "nonceStr": "abc123",
      "package": "prepay_id=wx123...",
      "signType": "RSA",
      "paySign": "..."
    }
  }
}
```

---

### 17.2 微信支付回调（内部）

```http
POST /payment/wx-notify
```

**说明**: 此接口由微信服务器调用，非客户端调用。服务端验签后发放钻石。

---

### 17.3 查询订单状态

```http
GET /payment/orders/:orderId
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "orderId": "pay_order_001",
    "status": "paid",
    "packageId": "pkg_30yuan",
    "diamondsGranted": 360,
    "paidAt": "2026-02-28T10:30:05Z"
  }
}
```

---

### 17.4 订阅月卡

```http
POST /payment/subscribe
🔒 需要认证
```

**请求体**

```json
{
  "planId": "monthly",
  "platform": "miniprogram"
}
```

| planId | 价格 | 周期 |
|--------|------|------|
| `monthly` | ¥30 | 1个月 |
| `yearly` | ¥258 | 12个月 |

---

## 18. 排行榜 Ranking

### 18.1 个人排行榜

```http
GET /ranking/personal
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `dimension` | string | `output`（产值）/ `social`（社交）/ `collection`（收藏）/ `quality`（品质）/ `total`（综合，默认） |
| `scope` | string | `village`（默认）/ `global` |
| `page` | int | 默认 1 |

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "seasonId": "season_001",
    "seasonName": "第一赛季",
    "myRank": 3,
    "myScore": 12450,
    "list": [
      {
        "rank": 1,
        "userId": "usr_def456",
        "nickname": "小红",
        "avatar": "https://...",
        "score": 15800,
        "badge": "gold"
      },
      {
        "rank": 2,
        "userId": "usr_ghi789",
        "nickname": "小李",
        "score": 13200,
        "badge": "silver"
      },
      {
        "rank": 3,
        "userId": "usr_abc123",
        "nickname": "小明",
        "score": 12450,
        "badge": "bronze",
        "isMe": true
      }
    ],
    "total": 12,
    "page": 1,
    "pageSize": 20
  }
}
```

---

### 18.2 村庄排行榜

```http
GET /ranking/village
🔒 需要认证
```

**Query 参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| `dimension` | string | `output` / `project` / `social` / `activity` / `total`（默认）|
| `page` | int | 默认 1 |

**响应**（结构类似 18.1，列表中条目为村庄信息）

---

## 19. 赛季 Season

### 19.1 获取当前赛季信息

```http
GET /season/current
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "seasonId": "season_001",
    "name": "第一赛季",
    "startAt": "2026-02-01T00:00:00Z",
    "endAt": "2026-02-29T23:59:59Z",
    "daysLeft": 1,
    "myScore": 12450,
    "myRank": 3,
    "scoreBreakdown": {
      "output": 4980,
      "social": 3735,
      "collection": 2490,
      "quality": 1245
    },
    "tasks": [
      {
        "id": "task_001",
        "name": "收获 50 次",
        "progress": 48,
        "target": 50,
        "reward": { "type": "coins", "quantity": 500 },
        "claimed": false
      }
    ],
    "rewards": {
      "top1": { "diamonds": 500, "title": "传奇农夫", "decoration": "legend_frame" },
      "top5": { "diamonds": 300, "title": "史诗农夫" },
      "top20": { "diamonds": 100, "title": "精英农夫" },
      "participation": { "decoration": "season1_frame" }
    }
  }
}
```

---

### 19.2 领取赛季任务奖励

```http
POST /season/task/:taskId/claim
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "reward": { "type": "coins", "quantity": 500 },
    "coinsTotal": 9500
  }
}
```

---

### 19.3 历史赛季

```http
GET /season/history
🔒 需要认证
```

**响应**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      {
        "seasonId": "season_000",
        "name": "测试赛季",
        "myFinalRank": 5,
        "myFinalScore": 9800,
        "reward": { "diamonds": 300, "title": "史诗农夫" },
        "endAt": "2026-01-31T23:59:59Z"
      }
    ]
  }
}
```

---

## 20. WebSocket 事件

### 20.1 连接

```
wss://api.nongqucun.com/ws?token=<JWT>
```

连接成功后服务端发送确认：

```json
{
  "type": "connected",
  "data": {
    "userId": "usr_abc123",
    "villageId": "vil_xyz789",
    "serverTime": "2026-02-28T10:30:00Z"
  }
}
```

### 20.2 心跳

客户端每 30 秒发送 ping，服务端回复 pong：

```json
// 客户端发送
{ "type": "ping", "ts": 1709123456789 }

// 服务端回复
{ "type": "pong", "ts": 1709123456789 }
```

### 20.3 服务端推送事件类型

所有推送事件格式：

```json
{
  "type": "<event_type>",
  "data": { },
  "ts": "2026-02-28T10:30:00Z"
}
```

**事件类型列表**

| type | 触发时机 | data 结构 |
|------|----------|-----------|
| `farm_tick` | 每 5 分钟 Tick 完成后 | `{ updatedPlots: Plot[], coinsEarned: number }` |
| `crop_mature` | 作物成熟 | `{ plotX, plotY, cropId, cropName }` |
| `social_visit` | 他人 Agent 来访 | `{ visitorNickname, agentName, summary }` |
| `social_gift_received` | 收到赠礼 | `{ fromNickname, itemName, quantity, affinityGained }` |
| `help_request` | 收到求助 | `{ requestId, fromNickname, resourceName, quantity }` |
| `help_response` | 求助有人响应 | `{ requestId, fromNickname, accepted, itemReceived? }` |
| `trade_sold` | 商品售出 | `{ orderId, itemName, quantity, coinsEarned }` |
| `trade_new` | 好友新商品上架 | `{ orderId, sellerNickname, itemName, price }` |
| `friend_request` | 收到好友申请 | `{ requestId, fromNickname, message }` |
| `affinity_level_up` | 关系等级提升 | `{ targetUserId, targetNickname, newLevel }` |
| `village_announce` | 村庄公告 | `{ content, publisherNickname }` |
| `village_project_complete` | 共建任务完成 | `{ projectName, villageNewLevel? }` |
| `random_event` | 随机事件发生 | `{ eventType, title, content, requiresAction: bool }` |
| `notification` | 通用通知 | `{ notificationId, title, content, actionType }` |
| `building_complete` | 建造/升级完成 | `{ buildingId, buildingType, newLevel }` |
| `season_ending` | 赛季即将结束 | `{ seasonName, hoursLeft }` |

### 20.4 随机事件处理（客户端响应）

收到 `requiresAction: true` 的随机事件后，玩家可通过 HTTP 响应：

```http
POST /event/:eventId/respond
🔒 需要认证
```

**请求体**

```json
{
  "action": "rescue"
}
```

（具体 `action` 值由事件类型决定，如暴风雨：`rescue` / `ignore`；动物走失：`search` / `ignore`）

---

*文档结束*

> **变更记录**
>
> | 版本 | 日期 | 变更内容 |
> |------|------|----------|
> | v1.0 | 2026-02-28 | 初稿，基于 PRD v1.0 定义全部接口 |
