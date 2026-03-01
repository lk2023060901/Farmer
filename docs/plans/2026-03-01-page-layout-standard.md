# 页面布局规范 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 统一所有 19 个页面的容器高度、ScrollView 底部间距、z-index 层级，并将 4 个页面的内联 Sheet/Toast 替换为共享组件。

**Architecture:** CSS Token（`:root` 变量）驱动，页面代码引用变量而非裸数字；共享 BottomSheet/Modal/Toast 组件替换各页面内联实现。

**Tech Stack:** Taro 3, React 18, TypeScript, SCSS Modules

---

## Task 1: 扩充 CSS Token

**Files:**
- Modify: `client/src/app.scss`

**Step 1: 在 `:root` 中增加 `--page-bottom` 和 z-index 变量**

找到现有 `:root` 块（约第 24 行），替换为：

```scss
:root {
  --tab-bar-height: 50px;
  --page-bottom: calc(var(--tab-bar-height) + 24px); /* 74px */

  /* Z-index 层级 */
  --z-modal:  1000;
  --z-sheet:  1200;
  --z-toast:  3000;
}
```

**Step 2: 验证**

`--page-bottom` 在浏览器 DevTools 中计算值应为 `74px`。

**Step 3: Commit**

```bash
git add client/src/app.scss
git commit -m "style: add --page-bottom and z-index CSS tokens"
```

---

## Task 2: Modal 与 BottomSheet 对齐 z-index 变量

**Files:**
- Modify: `client/src/components/ui/Modal/index.tsx:22`
- Modify: `client/src/components/ui/BottomSheet/index.tsx:25`

**Step 1: Modal — 将 `zIndex: 1000` 改为引用变量**

```tsx
// Modal/index.tsx, backdropStyle 内
zIndex: 'var(--z-modal)' as unknown as number,
```

**Step 2: BottomSheet — 将 `zIndex: 1200` 改为引用变量**

```tsx
// BottomSheet/index.tsx, overlayStyle 内
zIndex: 'var(--z-sheet)' as unknown as number,
```

**Step 3: Commit**

```bash
git add client/src/components/ui/Modal/index.tsx \
        client/src/components/ui/BottomSheet/index.tsx
git commit -m "style: align Modal/BottomSheet z-index to CSS tokens"
```

---

## Task 3: 新建共享 Toast 组件

**Files:**
- Create: `client/src/components/ui/Toast/index.tsx`

**Step 1: 创建文件**

```tsx
import React, { useEffect } from 'react';
import { View, Text } from '@tarojs/components';

interface ToastProps {
  message: string;
  visible: boolean;
  /** 显示时长 ms，默认 2000 */
  duration?: number;
  onHide: () => void;
}

const Toast: React.FC<ToastProps> = ({ message, visible, duration = 2000, onHide }) => {
  useEffect(() => {
    if (!visible) return;
    const t = setTimeout(onHide, duration);
    return () => clearTimeout(t);
  }, [visible, duration, onHide]);

  if (!visible) return null;

  return (
    <View
      style={{
        position: 'fixed',
        bottom: 'calc(var(--tab-bar-height) + 16px)',
        left: '50%',
        transform: 'translateX(-50%)',
        background: 'rgba(0,0,0,0.75)',
        color: '#fff',
        borderRadius: 20,
        padding: '8px 20px',
        fontSize: 14,
        zIndex: 'var(--z-toast)' as unknown as number,
        whiteSpace: 'nowrap',
        pointerEvents: 'none',
      }}
    >
      <Text style={{ color: '#fff', fontSize: 14 } as unknown as string}>{message}</Text>
    </View>
  );
};

export default Toast;
```

**Step 2: Commit**

```bash
git add client/src/components/ui/Toast/index.tsx
git commit -m "feat: add shared Toast component"
```

---

## Task 4: Inventory 页面 — 替换内联组件 + 修正容器

**Files:**
- Modify: `client/src/pages/inventory/index.tsx`

当前问题：
- 容器 `minHeight: '100vh'`（未减去 tab bar）
- 内联 `SellSheet`（约 90 行）
- 内联 `Toast`（约 15 行）

**Step 1: 替换 import，删除内联组件**

在文件顶部 import 区增加：
```tsx
import BottomSheet from '../../components/ui/BottomSheet';
import Toast from '../../components/ui/Toast';
```

删除整个 `SellSheet` 组件定义（第 28-119 行）和内联 `Toast` 定义（第 130-144 行）。

**Step 2: 将 SellSheet 内容改写为 BottomSheet children**

在 `InventoryPage` 内，将原来的 `{selling && <SellSheet .../>}` 替换为：

```tsx
<BottomSheet
  open={!!selling}
  onClose={() => setSelling(null)}
  title={selling ? `出售 ${itemEmoji(selling.itemId, selling.itemType)} ${itemName(selling.itemId)}` : ''}
>
  {selling && <SellContent
    itemId={selling.itemId}
    itemType={selling.itemType}
    maxQty={selling.qty}
    onClose={() => setSelling(null)}
    onSold={(coins) => setToast(`出售成功 +${coins} 金！`)}
  />}
</BottomSheet>
```

同时，在文件内保留一个精简版 `SellContent`（只含 qty stepper + 按钮，无外层 overlay）：

```tsx
const SellContent: React.FC<SellSheetProps> = ({ itemId, itemType, maxQty, onClose, onSold }) => {
  const sell = useInventoryStore((s) => s.sell);
  const setUser = useUserStore((s) => s.setUser);
  const [qty, setQty] = useState(1);
  const [busy, setBusy] = useState(false);
  const price = SELL_PRICES[itemId] ?? 5;
  const total = qty * price;

  const doSell = async () => {
    setBusy(true);
    try {
      const result = await sell(itemId, qty);
      setUser({ coins: result.newCoins });
      onSold(result.totalCoins);
      onClose();
    } finally {
      setBusy(false);
    }
  };

  return (
    <View>
      <Text style={{ fontSize: 13, color: '#8d6e63', display: 'block', marginBottom: 16 } as unknown as string}>
        单价 {price} 金 · 库存 {maxQty} 个
      </Text>
      <View style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 20 }}>
        <View onClick={() => setQty((q) => Math.max(1, q - 1))} style={stepBtn}>
          <Text style={{ color: '#fff', fontSize: 18, fontWeight: 700 } as unknown as string}>−</Text>
        </View>
        <Text style={{ fontSize: 22, fontWeight: 700, color: '#4e342e', minWidth: 40, textAlign: 'center' } as unknown as string}>
          {qty}
        </Text>
        <View onClick={() => setQty((q) => Math.min(maxQty, q + 1))} style={stepBtn}>
          <Text style={{ color: '#fff', fontSize: 18, fontWeight: 700 } as unknown as string}>+</Text>
        </View>
        <View onClick={() => setQty(maxQty)} style={{ ...stepBtn, background: '#8d6e63', padding: '6px 14px' }}>
          <Text style={{ color: '#fff', fontSize: 13 } as unknown as string}>全部</Text>
        </View>
      </View>
      <View
        onClick={!busy ? doSell : undefined}
        style={{ background: busy ? '#bdbdbd' : '#558b2f', borderRadius: 12, padding: '14px 0', textAlign: 'center', cursor: busy ? 'not-allowed' : 'pointer' }}
      >
        <Text style={{ color: '#fff', fontSize: 16, fontWeight: 700 } as unknown as string}>
          {busy ? '出售中…' : `出售 ${qty} 个 → +${total} 金`}
        </Text>
      </View>
    </View>
  );
};
```

**Step 3: 替换内联 Toast**

```tsx
{/* 原来: {toast && <Toast msg={toast} onDone={() => setToast('')} />} */}
<Toast message={toast} visible={!!toast} onHide={() => setToast('')} />
```

**Step 4: 修正外层容器高度**

```tsx
// 原: style={{ background: '#fdf6e3', minHeight: '100vh' }}
style={{ background: '#fdf6e3', minHeight: 'calc(100vh - var(--tab-bar-height))' }}
```

**Step 5: 修正 ScrollView padding**

```tsx
// 原: <ScrollView scrollY style={{ padding: 16 }}>
<ScrollView scrollY style={{ padding: '16px 16px var(--page-bottom)' }}>
```

**Step 6: Commit**

```bash
git add client/src/pages/inventory/index.tsx
git commit -m "refactor(inventory): use shared BottomSheet/Toast, fix container height"
```

---

## Task 5: Trade 页面 — 替换内联组件 + 修正容器

**Files:**
- Modify: `client/src/pages/trade/index.tsx`

当前问题：
- 容器 `minHeight: '100vh'`
- 内联 `BuySheet`、`ListSheet`、`Toast`

**Step 1: 增加 import**

```tsx
import BottomSheet from '../../components/ui/BottomSheet';
import Toast from '../../components/ui/Toast';
```

**Step 2: 删除内联 Toast（第 34-48 行）**

**Step 3: 将 BuySheet 改为 BottomSheet**

删除 `BuySheet` 组件定义（第 52-154 行）中的外层 fixed overlay，保留内部逻辑为 `BuyContent`。使用处改为：

```tsx
<BottomSheet
  open={!!buying}
  onClose={() => setBuying(null)}
  title={buying ? `购买 ${itemEmoji(buying.itemId, buying.itemType)} ${itemLabel(buying.itemId)}` : ''}
>
  {buying && <BuyContent
    order={buying}
    userCoins={coins}
    isMine={buying.sellerId === myId}
    onClose={() => setBuying(null)}
    onBought={(cost) => { addCoins(-cost); setToast(`购买成功！花费 ${cost} 金`); }}
  />}
</BottomSheet>
```

**Step 4: 将 ListSheet 改为 BottomSheet**

```tsx
<BottomSheet
  open={listing}
  onClose={() => setListing(false)}
  title="🏪 上架物品"
>
  <ListContent
    onClose={() => setListing(false)}
    onListed={(itemId) => setToast(`${itemLabel(itemId)} 已成功上架！`)}
  />
</BottomSheet>
```

**Step 5: 替换内联 Toast**

```tsx
<Toast message={toast} visible={!!toast} onHide={() => setToast('')} />
```

**Step 6: 修正容器**

```tsx
// 外层容器
style={{ background: '#fff8e1', minHeight: 'calc(100vh - var(--tab-bar-height))' }}

// ScrollView
style={{ padding: '16px 16px var(--page-bottom)' }}
```

**Step 7: Commit**

```bash
git add client/src/pages/trade/index.tsx
git commit -m "refactor(trade): use shared BottomSheet/Toast, fix container height"
```

---

## Task 6: Village 页面 — 替换内联组件 + 修正容器

**Files:**
- Modify: `client/src/pages/village/index.tsx`

当前问题：
- 容器 `minHeight: '100vh'`
- 内联 `ContributeSheet`、`CreateVillageModal`、`Toast`

**Step 1: 增加 import**

```tsx
import BottomSheet from '../../components/ui/BottomSheet';
import Modal from '../../components/ui/Modal';
import Toast from '../../components/ui/Toast';
```

**Step 2: 删除内联 Toast（前 ~15 行）**

**Step 3: ContributeSheet → BottomSheet**

删除外层 fixed overlay，保留内容为 `ContributeContent`：

```tsx
<BottomSheet
  open={!!contributing}
  onClose={() => setContributing(null)}
  title={contributing ? `${projectTypeEmoji(contributing.type)} 捐献 · ${contributing.name}` : ''}
>
  {contributing && <ContributeContent
    project={contributing}
    userCoins={userCoins}
    onClose={() => setContributing(null)}
    onDone={(coins) => setToast(`已捐献 ${coins} 金！`)}
  />}
</BottomSheet>
```

**Step 4: CreateVillageModal → Modal**

```tsx
<Modal
  open={creatingVillage}
  onClose={() => setCreatingVillage(false)}
  title="🏘️ 创建村庄"
>
  <CreateVillageContent onClose={() => setCreatingVillage(false)} />
</Modal>
```

**Step 5: 替换内联 Toast**

```tsx
<Toast message={toast} visible={!!toast} onHide={() => setToast('')} />
```

**Step 6: 修正容器和 ScrollView**

```tsx
style={{ background: '#f1f8e9', minHeight: 'calc(100vh - var(--tab-bar-height))' }}
// ScrollView padding
style={{ padding: '16px 16px var(--page-bottom)' }}
```

**Step 7: Commit**

```bash
git add client/src/pages/village/index.tsx
git commit -m "refactor(village): use shared BottomSheet/Modal/Toast, fix container height"
```

---

## Task 7: Visit 页面 — 替换内联组件 + 修正容器

**Files:**
- Modify: `client/src/pages/visit/index.tsx`

**Step 1: 增加 import**

```tsx
import BottomSheet from '../../components/ui/BottomSheet';
import Toast from '../../components/ui/Toast';
```

**Step 2: WaterSheet → BottomSheet（保留 WaterContent 内容部分）**

**Step 3: 替换内联 Toast**

```tsx
<Toast message={toast} visible={!!toast} onHide={() => setToast('')} />
```

**Step 4: 修正容器（visit 有返回按钮，无 tab bar，但全高即可）**

```tsx
style={{ minHeight: '100vh', background: '#f1f8e9' }}
```

Note: visit 是带返回按钮的页面（无 tab bar），保持 `100vh` 即正确。

**Step 5: Commit**

```bash
git add client/src/pages/visit/index.tsx
git commit -m "refactor(visit): use shared BottomSheet/Toast"
```

---

## Task 8: 批量修正滚动页面容器高度

以下页面只需修正外层容器 + ScrollView padding，无内联组件替换。

**Files（每个只改 1-2 处）:**

| 页面 | 容器当前值 | ScrollView padding 当前值 |
|------|------|------|
| `profile/index.tsx` | `minHeight: '100vh'` | `paddingBottom: 32`（无 ScrollView） |
| `shop/index.tsx` | `minHeight: '100vh'` | `padding: 16` |
| `animals/index.tsx` | `minHeight: '100vh'` | 无 |
| `collection/index.tsx` | `minHeight: '100vh'` | `height: calc(100vh - 220px)` |
| `daily/index.tsx` | `minHeight: '100vh'` | 无底部 padding |
| `workshop/index.tsx` | `minHeight: '100vh'` | `height: calc(100vh - 140px)` |
| `monthly_card/index.tsx` | 直接 ScrollView | 无底部 padding |

**统一替换规则：**

```tsx
// 外层容器
minHeight: '100vh'  →  minHeight: 'calc(100vh - var(--tab-bar-height))'

// ScrollView 底部
padding: N  →  padding: '16px 16px var(--page-bottom)'
// 或仅底部:
paddingBottom: N  →  paddingBottom: 'var(--page-bottom)'
```

**Step 1: 逐文件修改**（每个文件独立搜索 `minHeight: '100vh'` 替换）

**Step 2: Commit**

```bash
git add client/src/pages/profile/index.tsx \
        client/src/pages/shop/index.tsx \
        client/src/pages/animals/index.tsx \
        client/src/pages/collection/index.tsx \
        client/src/pages/daily/index.tsx \
        client/src/pages/workshop/index.tsx \
        client/src/pages/monthly_card/index.tsx
git commit -m "style: standardize scroll page container height and bottom padding"
```

---

## Task 9: SCSS 页面容器修正

以下页面使用 SCSS 类，需在对应 `.scss` 文件中添加标准容器规则。

**Files:**
- `client/src/pages/chat/index.scss`（或同名模块文件）
- `client/src/pages/season/index.scss`
- `client/src/pages/settings/index.scss`
- `client/src/pages/share/index.scss`

**Step 1: 找到每个页面的根容器 SCSS 类，添加：**

```scss
.page-root {  /* 替换为实际类名 */
  min-height: calc(100vh - var(--tab-bar-height));
}

.page-scroll {  /* 替换为实际 scroll 容器类名 */
  padding-bottom: var(--page-bottom);
}
```

**Step 2: Commit**

```bash
git add client/src/pages/chat/ \
        client/src/pages/season/ \
        client/src/pages/settings/ \
        client/src/pages/share/
git commit -m "style: fix SCSS-based page container heights"
```

---

## 验证清单

- [ ] 所有滚动页最底部内容距离 tab bar 至少 24px
- [ ] BottomSheet 打开时不被 tab bar 遮挡（z-index 正确）
- [ ] Toast 显示在 tab bar 上方
- [ ] Farm 页 Canvas 高度不变（仍用 `calc(100vh - var(--tab-bar-height))`）
- [ ] 小程序构建 `yarn build:weapp` 无报错
- [ ] H5 开发 `yarn dev:h5` 控制台无类型报错
