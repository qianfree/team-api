# Team API Console 设计系统文档

## 一、技术栈

| 类别 | 技术 | 版本 |
|------|------|------|
| 框架 | Vue 3 | 3.5.32 |
| 类型系统 | TypeScript | 6.0.2 |
| 构建工具 | Vite | 6.4.1 |
| UI 组件库 | Arco Design Vue | 2.57.0 |
| 状态管理 | Pinia | 3.0.4 |
| 路由 | Vue Router | 5.0.4 |
| HTTP 客户端 | Axios | 1.14.0 |
| 图表 | ECharts + Vue ECharts | 6.0.0 / 8.0.1 |

---

## 二、配色方案（核心）

### 主色系 — Teal 青绿色

```
主色:        #0d9488 (Teal-600)
主色悬浮:    #0f766e (Teal-700)
主色浅底:    #f0fdfa (Teal-50)
主色渐变:    linear-gradient(135deg, #14b8a6, #0d9488)
主色辉光:    0 0 20px rgba(13, 148, 136, 0.2)
```

### Arco 主题色阶

```
--color-primary-1:  #f0fdfa  (最浅)
--color-primary-2:  #99f6e4
--color-primary-3:  #5eead4
--color-primary-4:  #2dd4bf
--color-primary-5:  #14b8a6  (中)
--color-primary-6:  #0d9488  (默认主色)
--color-primary-7:  #0f766e  (深)
```

### 语义色

```
成功:  #10b981 (Green-500)
警告:  #f59e0b (Amber-500)
危险:  #ef4444 (Red-500)
信息:  #06b6d4 (Cyan-500)
```

### 文字色（亮色模式）

```
主文字:    #1e293b (Slate-800)
次文字:    #475569 (Slate-600)
辅助文字:  #94a3b8 (Slate-400)
最弱文字:  #cbd5e1 (Slate-300)
```

### 背景色（亮色模式）

```
页面底色:   #f8fafc (Slate-50)
卡片底色:   #ffffff
次级底色:   #f1f5f9 (Slate-100)
玻璃态底色: rgba(255, 255, 255, 0.8)
```

### 边框色

```
主边框:  #e2e8f0 (Slate-200)
轻边框:  #f1f5f9 (Slate-100)
```

### 侧边栏

```
背景:         #0f172a (Slate-900)
激活态:       rgba(13, 148, 136, 0.15)
文字:         rgba(255, 255, 255, 0.7)
激活文字:     #ffffff
分隔线:       rgba(255, 255, 255, 0.06)
```

### 统计卡片渐变色

```
Teal:   linear-gradient(135deg, #0d9488, #14b8a6)
Green:  linear-gradient(135deg, #10b981, #34d399)
Orange: linear-gradient(135deg, #f59e0b, #fbbf24)
Cyan:   linear-gradient(135deg, #06b6d4, #22d3ee)
Purple: linear-gradient(135deg, #8b5cf6, #a78bfa)
Red:    linear-gradient(135deg, #ef4444, #f87171)
```

### 暗色模式关键色

```
页面底色:   #020617 (Slate-950)
卡片底色:   #0f172a (Slate-900)
次级底色:   #1e293b (Slate-800)
主文字:     #f1f5f9
次文字:     #cbd5e1
边框:       #1e293b
```

### 暗色模式 Arco 覆盖

```
--color-bg-1:     #0f172a
--color-bg-2:     #020617
--color-bg-3:     #1e293b
--color-bg-4:     #1e293b
--color-bg-5:     #334155
--color-text-1:   #f1f5f9
--color-text-2:   #cbd5e1
--color-text-3:   #64748b
--color-text-4:   #475569
--color-border:   #1e293b
--color-border-2: #334155
--color-fill-2:   #1e293b
--color-fill-3:   #334155
```

---

## 三、阴影系统

```
默认:       0 1px 2px rgba(0,0,0,0.03), 0 4px 12px rgba(0,0,0,0.04)
悬浮:       0 8px 24px rgba(0,0,0,0.06), 0 2px 6px rgba(0,0,0,0.03)
卡片:       0 1px 3px rgba(0,0,0,0.04), 0 1px 2px rgba(0,0,0,0.06)
按钮:       0 4px 12px rgba(13,148,136,0.3)
按钮悬浮:   0 6px 16px rgba(13,148,136,0.4)
暗色默认:   0 1px 2px rgba(0,0,0,0.2), 0 4px 12px rgba(0,0,0,0.15)
暗色悬浮:   0 8px 24px rgba(0,0,0,0.25), 0 2px 6px rgba(0,0,0,0.15)
暗色卡片:   0 1px 3px rgba(0,0,0,0.15)
```

---

## 四、圆角系统

```
默认:     10px (--ta-radius)
大圆角:   14px (--ta-radius-lg)
超大圆角: 20px (--ta-radius-xl)
```

---

## 五、字体

```
字体族: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
       'Helvetica Neue', Arial, 'PingFang SC', 'Microsoft YaHei', sans-serif

Logo:       16px / 700
页面标题:   20px / 700
卡片标题:   18px / 600
菜单分组:   11px / 600 (大写)
菜单项:     13px / 500
正文:       13-15px / 400
统计数值:   24px / 700
表格头:     13px / 600
```

---

## 六、布局结构

```
┌─────────────────────────────────────────────┐
│ 侧边栏 (220px, 可折叠至 64px)  │ 主内容区    │
│                               │ (flex: 1)  │
│ ┌───────────────────────────┐ │            │
│ │ Logo (56px高)             │ │  padding:  │
│ ├───────────────────────────┤ │   24px     │
│ │ 菜单分组                   │ │            │
│ │  · 概览                    │ │  卡片布局   │
│ │  · 资源管理                 │ │  间距:16px │
│ │  · 财务中心                 │ │            │
│ │  · 安全审计                 │ │            │
│ │  · 系统                    │ │            │
│ ├───────────────────────────┤ │            │
│ │ 底部: 主题切换/用户/折叠    │ │            │
│ └───────────────────────────┘ │            │
└─────────────────────────────────────────────┘
```

### 侧边栏菜单项样式

```
内边距:       8px 12px
圆角:         8px
图标与文字间距: 10px
激活态背景:   rgba(13, 148, 136, 0.15)
激活态文字色: #5eead4
```

### 页面背景装饰

```
radial-gradient(at 40% 20%, rgba(13, 148, 136, 0.04) 0px, transparent 50%),
radial-gradient(at 80% 0%, rgba(6, 182, 212, 0.03) 0px, transparent 50%),
radial-gradient(at 0% 60%, rgba(13, 148, 136, 0.02) 0px, transparent 50%)
```

---

## 七、玻璃拟态 (Glassmorphism)

```
背景:   rgba(255, 255, 255, 0.8)    (亮色) / rgba(15, 23, 42, 0.85) (暗色)
边框:   rgba(255, 255, 255, 0.3)
模糊:   16px (--ta-glass-blur)
```

---

## 八、动效系统

### 过渡

```
缓动曲线: cubic-bezier(0.4, 0, 0.2, 1)
快速:     0.15s
正常:     0.2s
慢速:     0.25s
```

### 动画

```
fadeIn     — 透明度渐入
slideUp    — 向上滑入 + 渐入
slideDown  — 向下滑入 + 渐入
scaleIn    — 缩放 + 渐入
glow       — 脉冲辉光效果
float      — 浮动光球 (8s 循环)
cardAppear — 卡片入场 (0.5s)
```

---

## 九、自定义滚动条

```
宽度:         6px
轨道:         transparent
滑块(常态):   rgba(148, 163, 184, 0.4)
滑块(悬浮):   rgba(100, 116, 139, 0.5)
圆角:         3px
```

---

## 十、选中态

```
背景: rgba(13, 148, 136, 0.15)
文字: #042f2e
```

---

## 十一、图表配色 (ECharts)

```
分布图调色板: #165DFF, #0fc6c2, #f7ba1e, #f76560, #722ed1,
             #3491fa, #00b42a, #ff7d00, #eb2f96, #86909c

质量概览:
  成功: #00b42a
  失败: #f53f3f
```

---

## 十二、登录页专用色

### 背景

```
linear-gradient(135deg, #020617 0%, #0f172a 30%, #042f2e 60%, #0f172a 100%)
```

### 浮动光球

```
光球1: rgba(13, 148, 136, 0.12) — 300px
光球2: rgba(6, 182, 212, 0.08)  — 250px
光球3: rgba(20, 184, 166, 0.06) — 200px
```

### 登录卡片

```
背景:   rgba(255, 255, 255, 0.9)
模糊:   24px
圆角:   20px
阴影:   0 8px 32px rgba(0, 0, 0, 0.2)
```

### 品牌图标

```
渐变: linear-gradient(135deg, #14b8a6, #0d9488)
阴影: 0 4px 16px rgba(13, 148, 136, 0.4)
```

---

## 十三、卡片增强样式

```
圆角:     10px
边框:     无 (borderless)
阴影:     微阴影 + 悬浮抬升
悬浮效果:  box-shadow 加深 + 轻微上移
过渡:     cubic-bezier(0.4, 0, 0.2, 1) 0.25s
```

---

## 十四、表格增强样式

```
表头背景:   灰色底色
行悬浮:     Teal 淡色底
单元格边框: 底部边框
```

---

## 十五、按钮增强样式

```
主按钮:   Teal 渐变背景 + 辉光阴影
悬浮态:   阴影加深 + 轻微上移
按下态:   scale(0.97)
```

---

## 十六、输入框聚焦态

```
边框色:   #0d9488 (Teal)
辉光阴影: 0 0 0 3px rgba(13, 148, 136, 0.1)
```

---

## 十七、标签 (Tag) 样式

```
圆角:     6px
内边距:   2px 10px
字号:     12px
```

---


## 十九、设计特征总结

| 特征 | 描述 |
|------|------|
| **风格** | 现代简约 Dashboard，玻璃拟态 + 柔和阴影 |
| **主色调** | Teal 青绿 (#0d9488)，搭配 Slate 灰色系 |
| **动效** | 入场动画 (fadeIn/slideUp/scaleIn)，悬浮抬升，脉冲辉光 |
| **圆角** | 10px 默认，柔和但不圆润 |
| **暗色模式** | 完整支持，Slate-950 为底色 |
| **侧边栏** | 深色 Slate-900 背景，激活项带 Teal 半透明底色 |
| **卡片** | 无边框 + 微阴影 + 悬浮抬升效果 |
| **按钮** | 主按钮使用 Teal 渐变 + 辉光阴影 |

核心设计语言：**Teal (#0d9488) + Slate 灰色系**，辅以玻璃拟态效果和微动效，整体风格干净、专业、现代。
