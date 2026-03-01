# 页面布局规范设计文档

**日期**：2026-03-01
**范围**：`client/src/` — 所有 19 个页面 + 共享 UI 组件
**方案**：CSS Token + 共享组件（思路 1）

---

## 背景

当前问题：
- 只有 `farm` 页使用 `var(--tab-bar-height)`；其余 18 页用 `minHeight: 100vh`，内容可钻到 tab bar 下面
- ScrollView 底部 padding 各不相同（16px / 80px / 96px）
- `inventory`、`trade`、`village`、`visit` 各自内联实现了 Sheet 和 Toast，项目已有 `BottomSheet.tsx` / `Modal.tsx` 未充分利用
- `Modal`、`BottomSheet` 的 z-index 硬编码，无统一层级

---

## 一、CSS Token 标准

在 `src/app.scss` 的 `:root` 扩充变量：

```scss
:root {
  --tab-bar-height: 50px;
  --page-bottom: calc(var(--tab-bar-height) + 24px); /* 74px */

  /* Z-index 层级（从低到高）*/
  --z-modal:  1000;   /* Modal（居中遮罩）*/
  --z-sheet:  1200;   /* BottomSheet（底部抽屉）*/
  --z-toast:  3000;   /* Toast（始终最顶层）*/
}
```

---

## 二、页面容器规范

### 类型 A — 画布页（farm，共 1 页）

```tsx
// 不变，已正确
style={{ position: 'relative', width: '100%', height: 'calc(100vh - var(--tab-bar-height))' }}
```

### 类型 B — 滚动页（共 15 页）

受影响：village, trade, profile, shop, inventory, visit, animals, collection, daily, workshop, monthly_card, season, chat, settings, share

```tsx
// 外层容器
style={{ minHeight: 'calc(100vh - var(--tab-bar-height))', background: '...' }}

// ScrollView 或内容区底部
style={{ padding: '16px 16px var(--page-bottom)' }}
// 或仅需底部 padding：paddingBottom: 'var(--page-bottom)'
```

### 类型 C — 全屏静态页（共 3 页）

受影响：index, agreement, privacy

```tsx
// 无 tab bar，全高
style={{ height: '100vh' }}
// 或保持 minHeight: 100vh（这些页面本身正确）
```

---

## 三、共享组件规范

### 3.1 现有组件对齐变量

| 组件 | 文件 | 当前 z-index | 改为 |
|------|------|------|------|
| `Modal` | `components/ui/Modal/index.tsx` | `1000` | `var(--z-modal)` |
| `BottomSheet` | `components/ui/BottomSheet/index.tsx` | `1200` | `var(--z-sheet)` |

### 3.2 新建 Toast 组件

**路径**：`src/components/ui/Toast/index.tsx`

接口：
```tsx
interface ToastProps {
  message: string;
  visible: boolean;
}
```

样式：
- `position: fixed; bottom: calc(var(--tab-bar-height) + 16px); left: 50%; transform: translateX(-50%)`
- `zIndex: var(--z-toast)` (3000)
- 圆角胶囊，背景 `rgba(0,0,0,0.75)`，白色文字
- 外部控制 `visible`，内部不做自动消失（由页面 `useEffect` 控制）

---

## 四、内联 Sheet → 共享组件

| 页面 | 当前内联实现 | 替换为 |
|------|------|------|
| `inventory` | `SellSheet` (固定遮罩 + 内容) | `<BottomSheet>` |
| `inventory` | 内联 Toast | `<Toast>` |
| `trade` | `BuySheet` | `<BottomSheet>` |
| `trade` | `ListSheet` | `<BottomSheet>` |
| `trade` | 内联 Toast | `<Toast>` |
| `village` | `ContributeSheet` | `<BottomSheet>` |
| `village` | `CreateVillageModal` | `<Modal>` |
| `village` | 内联 Toast | `<Toast>` |
| `visit` | `WaterSheet` | `<BottomSheet>` |
| `visit` | 内联 Toast | `<Toast>` |

---

## 五、修改文件清单

| 文件 | 操作 |
|------|------|
| `src/app.scss` | 扩充 `:root` 变量 |
| `src/components/ui/Modal/index.tsx` | z-index → `var(--z-modal)` |
| `src/components/ui/BottomSheet/index.tsx` | z-index → `var(--z-sheet)` |
| `src/components/ui/Toast/index.tsx` | **新建** |
| `src/pages/village/index.tsx` | 容器 + 内联组件替换 |
| `src/pages/trade/index.tsx` | 容器 + 内联组件替换 |
| `src/pages/profile/index.tsx` | 容器高度 |
| `src/pages/shop/index.tsx` | 容器 + scroll padding |
| `src/pages/inventory/index.tsx` | 容器 + 内联组件替换 |
| `src/pages/visit/index.tsx` | 容器 + 内联组件替换 |
| `src/pages/animals/index.tsx` | 容器 + scroll padding |
| `src/pages/collection/index.tsx` | 容器 + scroll padding |
| `src/pages/daily/index.tsx` | 容器 + scroll padding |
| `src/pages/workshop/index.tsx` | 容器 + scroll padding |
| `src/pages/monthly_card/index.tsx` | 容器 + scroll padding |
| `src/pages/season/index.tsx` | 容器（SCSS 类） |
| `src/pages/chat/index.tsx` | 容器（SCSS 类） |
| `src/pages/settings/index.tsx` | 容器（SCSS 类） |
| `src/pages/share/index.tsx` | 容器（SCSS 类） |

**零改动**：`farm`（已正确）、`index`、`agreement`、`privacy`（静态页已正确）

---

## 六、验证标准

1. 所有滚动页最底部内容距离 tab bar 至少 24px 间距
2. 打开任意 BottomSheet / Modal，z-index 层级正确（不被 tab bar 遮挡）
3. Toast 显示在 tab bar 上方
4. Farm 页 canvas 高度不变
5. 小程序构建 `yarn build:weapp` 无报错
