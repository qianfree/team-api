# 租户控制台 — 前端设计语言规范

> 参考 sub2api 项目的设计系统，为租户控制台建立统一的设计语言。所有新页面和组件必须遵循本规范。

## 一、色彩系统

### 主色 — Teal/Cyan 青色系

| 级别 | 色值 | 用途 |
|------|------|------|
| primary-50 | `#f0fdfa` | 极浅背景、hover 状态 |
| primary-100 | `#ccfbf1` | 选中态背景、Badge 背景 |
| primary-200 | `#99f6e4` | 边框、分割线 |
| primary-300 | `#5eead4` | 装饰元素 |
| primary-400 | `#2dd4bf` | 次要强调 |
| **primary-500** | **`#14b8a6`** | **主色，按钮、链接、强调色** |
| primary-600 | `#0d9488` | 渐变终点、hover 加深 |
| primary-700 | `#0f766e` | Active 状态 |
| primary-800 | `#115e59` | 深色文字 |
| primary-900 | `#134e4a` | 深色文字、暗背景 |
| primary-950 | `#042f2e` | 极深背景 |

### 语义色

| 用途 | 浅色 | 深色 |
|------|------|------|
| 成功 | `emerald-500` / `bg-emerald-100 text-emerald-700` | `bg-emerald-900/30 text-emerald-400` |
| 警告 | `amber-500` / `bg-amber-100 text-amber-700` | `bg-amber-900/30 text-amber-400` |
| 错误 | `red-500` / `bg-red-100 text-red-700` | `bg-red-900/30 text-red-400` |
| 信息 | `primary-500` / `bg-primary-100 text-primary-700` | `bg-primary-900/30 text-primary-400` |
| 中性 | `bg-gray-100 text-gray-700` | `bg-dark-700 text-dark-300` |

### 中性色

使用 TailwindCSS 内置的 `gray` 系列（gray-50 ~ gray-950）。

### 深色模式（预留）

深色模式背景使用 `dark` 色板，与 `gray` 色板值相同但语义独立：
```
dark-50: #f8fafc → dark-950: #020617
```

### 渐变

| 名称 | 定义 | 用途 |
|------|------|------|
| gradient-primary | `linear-gradient(135deg, #14b8a6, #0d9488)` | 主按钮、品牌装饰 |
| gradient-dark | `linear-gradient(135deg, #1e293b, #0f172a)` | 深色背景 |
| mesh-gradient | 多层径向渐变（primary/cyan 透明色） | 页面背景装饰 |
| text-gradient | `from-primary-600 to-primary-500 bg-clip-text` | 品牌标题文字 |

---

## 二、排版

### 字体栈

```css
font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
  'Helvetica Neue', Arial, 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
```

等宽字体：
```css
font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
```

### 字号层级

| 用途 | 类名 | 大小 |
|------|------|------|
| 页面标题 | `text-2xl font-bold` | 24px 粗体 |
| 卡片标题 | `text-lg font-semibold` | 18px 半粗 |
| 正文 | `text-sm` | 14px |
| 辅助文字 | `text-xs` | 12px |
| 统计数值 | `text-2xl font-bold` | 24px 粗体 |
| 标签文字 | `text-xs font-medium` | 12px 中等 |

### 字重

| 名称 | 值 | 用途 |
|------|------|------|
| normal | 400 | 正文 |
| medium | 500 | 标签、按钮 |
| semibold | 600 | 小标题、导航 |
| bold | 700 | 页面标题、统计数值 |

---

## 三、间距与圆角

### 内边距规范

| 用途 | 值 |
|------|------|
| 页面内容区 | `p-4 md:p-6 lg:p-8` |
| 卡片内容 | `p-6` |
| 卡片头部/底部 | `px-6 py-4` |
| 输入框 | `px-4 py-2.5` |
| 模态框内边距 | `px-4 py-3 sm:px-6 sm:py-4` |
| 表格单元格 | `px-4 py-3` |

### 间距（gap）

- 组件内部元素：`gap-2` ~ `gap-3`
- 卡片/区块之间：`gap-4` ~ `gap-6`
- 页面区块之间：`gap-6` ~ `gap-8`

### 圆角

| 用途 | 类名 | 值 |
|------|------|------|
| 小元素（Badge、小按钮） | `rounded-lg` / `rounded-full` | 8px / 全圆 |
| 中元素（按钮、输入框、Tab） | `rounded-xl` | 12px |
| 大元素（卡片、模态框、面板） | `rounded-2xl` | 16px |

---

## 四、阴影体系

| 名称 | 值 | 用途 |
|------|------|------|
| shadow-card | `0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.06)` | 普通卡片 |
| shadow-card-hover | `0 10px 40px rgba(0,0,0,0.08)` | 卡片悬停 |
| shadow-glass | `0 8px 32px rgba(0,0,0,0.08)` | 玻璃拟态卡片 |
| shadow-glass-sm | `0 4px 16px rgba(0,0,0,0.06)` | 小型玻璃效果 |
| shadow-glow | `0 0 20px rgba(20,184,166,0.25)` | 主色发光 |
| shadow-glow-lg | `0 0 40px rgba(20,184,166,0.35)` | 大面积发光 |
| shadow-md (主色) | `shadow-md shadow-primary-500/25` | 主按钮阴影 |
| shadow-lg (主色) | `shadow-lg shadow-primary-500/30` | 主按钮悬停阴影 |

---

## 五、组件样式规范

### 5.1 按钮

基础样式 `.btn`：
```
inline-flex items-center justify-center gap-2
rounded-xl px-4 py-2.5 text-sm font-medium
transition-all duration-200 ease-out
focus:outline-none focus:ring-2 focus:ring-primary-500/50 focus:ring-offset-2
active:scale-[0.98]
```

变体：

| 类名 | 样式 |
|------|------|
| `.btn-primary` | 渐变背景 `from-primary-500 to-primary-600`，白色文字，主色阴影 |
| `.btn-secondary` | 白色背景，灰色边框，悬停变灰 |
| `.btn-ghost` | 透明背景，悬停灰色背景 |
| `.btn-danger` | 渐变 `from-red-500 to-red-600`，红色阴影 |
| `.btn-success` | 渐变 `from-emerald-500 to-emerald-600`，绿色阴影 |

尺寸：

| 类名 | 样式 |
|------|------|
| `.btn-sm` | `rounded-lg px-3 py-1.5 text-xs` |
| `.btn-md` | `rounded-xl px-4 py-2 text-sm`（默认） |
| `.btn-lg` | `rounded-2xl px-6 py-3 text-base` |
| `.btn-icon` | `rounded-xl p-2.5`（正方形图标按钮） |

### 5.2 输入框

基础样式 `.input`：
```
w-full rounded-xl px-4 py-2.5 text-sm
bg-white border border-gray-200
text-gray-900 placeholder:text-gray-400
transition-all duration-200
focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30
```

错误态：`.input-error` — `border-red-500 focus:ring-red-500/30`
标签：`.input-label` — `mb-1.5 block text-sm font-medium text-gray-700`
提示：`.input-hint` — `mt-1 text-xs text-gray-500`
错误文字：`.input-error-text` — `mt-1 text-xs text-red-500`

### 5.3 卡片

| 类名 | 样式 |
|------|------|
| `.card` | 白色背景，圆角 2xl，浅灰边框，shadow-card |
| `.card-hover` | 悬停上浮 2px + 加深阴影 |
| `.card-glass` | 半透明白色 + backdrop-blur-xl + shadow-glass |
| `.card-header` | 底部边框分割，px-6 py-4 |
| `.card-body` | p-6 |
| `.card-footer` | 顶部边框分割，px-6 py-4 |

### 5.4 表格

容器：`.table-container` — `overflow-x-auto rounded-xl border border-gray-200`
表格：`.table` — `w-full text-sm`
表头：`bg-gray-50 text-gray-600 font-medium border-b`
单元格：`px-4 py-3 text-gray-700 border-b border-gray-100`
行悬停：`hover:bg-gray-50`
末行：`border-b-0`

### 5.5 徽章

基础：`.badge` — `inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium`
变体：`.badge-primary` `.badge-success` `.badge-warning` `.badge-danger` `.badge-gray` `.badge-purple`

### 5.6 模态框

遮罩：`.modal-overlay` — `fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-2 sm:p-4`
内容：`.modal-content` — `rounded-2xl shadow-2xl border flex flex-col max-h-[85vh]`
头部：`.modal-header` — `border-b px-4 py-3 sm:px-6 sm:py-4 flex items-center justify-between`
标题：`.modal-title` — `text-lg font-semibold text-gray-900`
主体：`.modal-body` — `px-4 py-3 sm:px-6 sm:py-4 flex-1 overflow-y-auto`
底部：`.modal-footer` — `border-t px-4 py-3 sm:px-6 sm:py-4 flex items-center justify-end gap-3`

### 5.7 下拉菜单

容器：`.dropdown` — `absolute z-50 bg-white rounded-xl border shadow-lg py-1 origin-top-right animate-scale-in`
选项：`.dropdown-item` — `px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 cursor-pointer flex items-center gap-2`

### 5.8 侧边栏

侧边栏：`.sidebar` — `fixed inset-y-0 left-0 z-40 w-64 bg-white border-r flex flex-col transition-transform duration-300`
头部：`.sidebar-header` — `h-16 px-6 flex items-center gap-3 border-b`
导航：`.sidebar-nav` — `flex-1 overflow-y-auto px-3 py-4`
链接：`.sidebar-link` — `flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-gray-600 transition-all hover:bg-gray-100`
活跃：`.sidebar-link-active` — `bg-primary-50 text-primary-600 hover:bg-primary-100`
分区标题：`.sidebar-section-title` — `mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-gray-400`

### 5.9 页面头部

区块：`.page-header` — `mb-6`
标题：`.page-title` — `text-2xl font-bold text-gray-900`
描述：`.page-description` — `mt-1 text-sm text-gray-500`

### 5.10 统计卡片

容器：`.stat-card` — `card p-5 flex items-start gap-4`
图标：`.stat-icon` — `h-12 w-12 rounded-xl flex items-center justify-center text-xl`
图标变体：`.stat-icon-primary` `.stat-icon-success` `.stat-icon-warning` `.stat-icon-danger`
数值：`.stat-value` — `text-2xl font-bold text-gray-900 truncate`
标签：`.stat-label` — `text-sm text-gray-500`
趋势：`.stat-trend` — `mt-1 flex items-center gap-1 text-xs font-medium`

### 5.11 标签页

容器：`.tabs` — `flex gap-1 p-1 rounded-xl bg-gray-100`
标签：`.tab` — `rounded-lg px-4 py-2 text-sm font-medium text-gray-600 transition-all`
活跃：`.tab-active` — `bg-white text-gray-900 shadow-sm`

### 5.12 代码块

行内：`.code` — `font-mono text-sm bg-gray-100 rounded px-1.5 py-0.5 text-primary-600`
块级：`.code-block` — `font-mono text-sm bg-gray-900 text-gray-100 overflow-x-auto rounded-xl p-4`

---

## 六、动效规范

### 过渡时长

| 类型 | 时长 | 缓动 |
|------|------|------|
| 颜色/背景变化 | `duration-150` | ease-out |
| 位移/缩放 | `duration-200` | ease-out |
| 模态框打开 | `250ms` | ease-out |
| 模态框关闭 | `200ms` | ease-in |
| 侧边栏展开/收起 | `duration-300` | linear |
| 页面淡入 | `duration-200` ~ `300ms` | ease-out |

### 关键动画

| 名称 | 类名 | 效果 |
|------|------|------|
| 淡入 | `animate-fade-in` | fadeIn 0.3s ease-out |
| 上滑 | `animate-slide-up` | translateY(10px) → 0 + 淡入 |
| 下滑 | `animate-slide-down` | translateY(-10px) → 0 + 淡入 |
| 右滑入 | `animate-slide-in-right` | translateX(20px) → 0 + 淡入 |
| 缩放入 | `animate-scale-in` | scale(0.95) → 1 + 淡入 |
| 骨架屏闪烁 | `animate-shimmer` | backgroundPosition -200% → 200% |
| 发光脉冲 | `animate-glow` | boxShadow 交替变化 |

### 模态框过渡

```css
/* 遮罩 */
.modal-enter-active { transition: opacity 250ms ease-out; }
.modal-leave-active { transition: opacity 200ms ease-in; }
.modal-enter-from, .modal-leave-to { opacity: 0; }

/* 内容 */
.modal-enter-active .modal-content { transition: transform 250ms ease-out, opacity 250ms ease-out; }
.modal-leave-active .modal-content { transition: transform 200ms ease-in, opacity 200ms ease-in; }
.modal-enter-from .modal-content { transform: scale(0.95); opacity: 0; }
```

### 交互反馈

- 按钮：`active:scale-[0.98]`（按下微缩）
- 卡片悬停：`hover:-translate-y-0.5`（上浮 2px）
- 链接/文字：颜色过渡 `transition-colors duration-150`

---

## 七、图标系统

### 规范

- 使用集中式 `<Icon>` 组件，按名称引用 SVG path
- 基于 Heroicons 风格（24x24 viewBox，stroke-linecap/linejoin: round）
- 默认 `stroke-width: 1.5`，填充 `none`，描边 `currentColor`

### 尺寸映射

| size | 类名 | 像素 |
|------|------|------|
| xs | `h-3.5 w-3.5` | 14px |
| sm | `h-4 w-4` | 16px |
| md | `h-5 w-5` | 20px（默认） |
| lg | `h-6 w-6` | 24px |
| xl | `h-8 w-8` | 32px |

### 常用图标清单

| 名称 | 用途 | path 特征 |
|------|------|-----------|
| `dashboard` (grid) | 仪表盘 | 四个方块 |
| `key` | API 密钥 | 钥匙 |
| `chart` | 统计图表 | 柱状图 |
| `users` | 成员管理 | 多人 |
| `user` | 用户/个人 | 单人 |
| `userPlus` | 邀请成员 | 加号+人 |
| `building` | 组织/租户 | 建筑 |
| `document` | 文档 | 文件 |
| `bookOpen` | API 文档 | 打开的书 |
| `cog` / `settings` | 设置 | 齿轮 |
| `shield` | 安全/权限 | 盾牌 |
| `creditCard` | 计费/订阅 | 信用卡 |
| `logout` | 退出登录 | 箭头+门 |
| `chevronDown/Right/Left` | 折叠/展开 | V 形箭头 |
| `eye` / `eyeOff` | 密码显示/隐藏 | 眼睛 |
| `search` | 搜索 | 放大镜 |
| `plus` | 新增 | 加号 |
| `edit` | 编辑 | 铅笔 |
| `trash` | 删除 | 垃圾桶 |
| `check` / `checkCircle` | 成功/确认 | 勾 |
| `xCircle` | 错误/关闭 | 叉+圆 |
| `exclamationTriangle` | 警告 | 三角感叹号 |
| `infoCircle` | 提示 | i+圆 |
| `menu` | 移动端菜单 | 三条横线 |
| `x` | 关闭 | 叉号 |
| `link` | 链接/复制 | 链条 |
| `refresh` | 刷新 | 循环箭头 |
| `copy` | 复制 | 两个重叠方块 |

---

## 八、响应式设计

### 断点

| 名称 | 宽度 | 策略 |
|------|------|------|
| sm | ≥640px | 小屏手机横屏 |
| md | ≥768px | 平板竖屏 |
| lg | ≥1024px | 平板横屏/小笔记本 |
| xl | ≥1280px | 桌面显示器 |

### 策略

- **侧边栏**：桌面端固定展示（可折叠），移动端抽屉模式（overlay + 遮罩）
- **页面内边距**：`p-4 md:p-6 lg:p-8` 渐进增大
- **表格**：桌面端完整表格，移动端可横向滚动（`overflow-x-auto`）或卡片视图
- **模态框**：移动端全宽 + 小内边距，桌面端居中限宽
- **网格**：`grid-cols-1 sm:grid-cols-2 lg:grid-cols-4` 渐进展开
- **头部标题**：桌面端显示页面标题，移动端隐藏（节省空间）

---

## 九、玻璃拟态（Glass Morphism）

### 参数

```css
/* 玻璃面板 */
background: rgba(255, 255, 255, 0.70);  /* 或 0.80 */
backdrop-filter: blur(24px);             /* backdrop-blur-xl */
border: 1px solid rgba(255, 255, 255, 0.20);
box-shadow: 0 8px 32px rgba(0, 0, 0, 0.08);
border-radius: 16px;                     /* rounded-2xl */
```

### 使用场景

| 场景 | 类名 | 说明 |
|------|------|------|
| 认证页卡片 | `.card-glass` | 登录/注册表单容器 |
| 头部导航 | `.glass` | `bg-white/80 backdrop-blur-xl`，sticky 定位 |
| 浮动面板 | `.glass-card` | 下拉菜单、弹出层 |
| 装饰背景 | mesh-gradient | 多层径向渐变 + blur |

### 深色模式适配

```css
dark:bg-dark-800/70   /* 玻璃面板背景 */
dark:border-dark-700/50 /* 玻璃边框 */
```

---

## 十、认证页布局

### 背景结构

```
渐变底色: bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100
装饰光球: 3 个大尺寸圆形（h-80/h-96 w-80/w-96）
  - 右上角: bg-primary-400/20 blur-3xl
  - 左下角: bg-primary-500/15 blur-3xl
  - 居中: bg-primary-300/10 blur-3xl
网格纹理: 64px 间距，primary/3% 透明度
```

### 内容结构

```
Logo（16x16 圆角方形，shadow-glow）
品牌名（text-gradient 渐变文字，text-3xl font-bold）
副标题（text-sm text-gray-500）
玻璃卡片容器（card-glass rounded-2xl p-8 shadow-glass）
  └── <slot /> 表单内容
底部链接（<slot name="footer" />）
版权信息（text-xs text-gray-400）
```

---

## 十一、滚动条

### 全局策略

默认隐藏，悬停/聚焦时显示细滚动条：

```css
* { scrollbar-width: thin; scrollbar-color: transparent transparent; }
*:hover, *:focus-within { scrollbar-color: rgba(156, 163, 175, 0.5) transparent; }
```

### 表格容器

始终显示滚动条：

```css
.table-wrapper { scrollbar-width: auto; scrollbar-color: rgba(156, 163, 175, 0.7) transparent; }
```

---

## 十二、加载与空状态

### 加载旋转器

`.spinner` — `h-5 w-5 rounded-full border-2 border-current border-t-transparent animate-spin`

### 骨架屏

`.skeleton` — `animate-pulse rounded bg-gray-200`

### 空状态

```
.empty-state        — flex flex-col items-center justify-center px-4 py-12 text-center
.empty-state-icon   — mb-4 h-16 w-16 text-gray-300
.empty-state-title  — mb-1 text-lg font-medium text-gray-900
.empty-state-description — max-w-sm text-sm text-gray-500
```

---

## 十三、TailwindCSS 自定义配置

在 TailwindCSS v4 中通过 `@theme` 指令或 CSS 自定义属性扩展：

### 需要自定义的扩展项

1. **colors.primary** — 完整的 teal 色阶（50~950）
2. **colors.dark** — 深色模式色阶
3. **boxShadow** — glass, glass-sm, glow, glow-lg, card, card-hover, inner-glow
4. **backgroundImage** — gradient-primary, gradient-dark, mesh-gradient
5. **animation** — fade-in, slide-up, slide-down, slide-in-right, scale-in, shimmer, glow
6. **keyframes** — 对应动画的关键帧定义



#### API统一响应格式

所有接口使用统一的 JSON 响应结构，成功和错误共享同一顶层字段。HTTP 状态码照常设置（不全部返回 200），但前端以响应体中的 `code` 字段为唯一判断依据。

**统一结构**：

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... },
  "request_id": "req_xxxxx"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | `0` = 成功，非 `0` = 错误。标准 HTTP 错误直接用状态码（400/401/403/404/409/500），业务错误用 >= 10000 的自定义码 |
| `message` | string | 用户可读的中文提示。成功时固定 `"ok"`，错误时描述具体原因，禁止暴露技术细节 |
| `data` | any | 成功时为业务数据（对象/数组/字符串），错误时为 `null` |
| `request_id` | string | 每个请求的唯一标识，贯穿全链路，用于日志追踪 |

**示例**：

```json
// 成功（HTTP 200）
{"code": 0, "message": "ok", "data": {"id": 1, "name": "张三"}, "request_id": "req_abc123"}

// 参数错误（HTTP 400）
{"code": 400, "message": "用户名不能为空", "data": null, "request_id": "req_def456"}

// 业务错误（HTTP 422）
{"code": 10001, "message": "余额不足", "data": null, "request_id": "req_ghi789"}
```

#### HTTP 状态码映射规则

后端必须同时设置正确的 HTTP 状态码（不全部返回 200），前端以 `code` 为主、HTTP 状态码为辅。

| 业务场景 | HTTP 状态码 | code 值 | 说明 |
|----------|------------|---------|------|
| 成功 | 200 | `0` | 请求处理成功 |
| 参数校验失败 | 400 | `400` | 请求体格式错误、必填字段缺失、值不合法 |
| 未认证 | 401 | `401` | Token 缺失、过期、无效 |
| 无权限 | 403 | `403` | 已认证但无权访问该资源 |
| 资源不存在 | 404 | `404` | 查询的对象不存在 |
| 乐观锁冲突 | 409 | `409` | 并发修改版本号冲突 |
| 请求频率超限 | 429 | `429` | 触发限流 |
| 业务规则错误 | 422 | `>= 10000` | 业务逻辑不满足，使用自定义错误码（见 `consts.go`） |
| 服务器内部错误 | 500 | `500` | 未预期的异常 |

#### 业务错误码定义（>= 10000）

业务错误码定义在 `internal/consts/consts.go` 中，每个错误码有对应的中文消息常量。新增业务错误时必须同时在 `consts.go` 中添加 `Code` 和 `Msg` 常量。

| 错误码 | 常量名 | 默认消息 |
|--------|--------|---------|
| 10001 | `CodeInsufficientBalance` | 余额不足 |
| 10002 | `CodeQuotaExceeded` | 额度已用完 |
| 10003 | `CodeChannelUnavailable` | 没有可用的渠道 |
| ... | 更多见 `consts.go` | ... |

#### 后端开发规范

- **统一使用 `internal/response` 包**返回响应，禁止在 handler 中直接拼接 `g.Map`
- 成功响应：`response.Success(r, data)` — 自动设置 HTTP 200 + `{"code": 0, "message": "ok", "data": ..., "request_id": ...}`
- 错误响应：`response.Error(r, err)` — 自动从 GoFrame `gerror` 提取 code，设置对应 HTTP 状态码
- 业务错误码 >= 10000 映射为 HTTP 422，标准 HTTP 错误码直接使用原值
- `message` 必须是用户可读的中文提示，禁止返回技术细节（SQL 错误、堆栈信息等）
- `request_id` 由中间件注入 Context，response 包自动读取，无需手动传递

#### 前端开发规范

- Axios 响应拦截器以 `response.data.code` 为唯一判断依据：`code === 0` 为成功，否则为失败
- 拦截器统一处理错误提示（读取 `response.data.message`），业务代码中不再逐个 catch 显示错误
- 所有 API 调用必须使用 `try/catch` 包裹
- 禁止在 API 调用的 try 块中直接显示成功提示（`Message.success`），必须确认请求真正成功后再显示