# IFRS 16 系统 UI 升级计划：Threads 极简风格

## 1. 当前 UI 设计审查 (Current UI Review)

通过分析你目前在 `@[web]` (Next.js) 中的代码（特别是 `AppLayout.tsx` 和默认的 Ant Design 配置），以及你在桌面的 `lease_management_ui` 概念设计，我总结了以下几点：

- **当前 Web 项目 (Ant Design 默认)**：目前采用的是非常标准的 B2B 企业级后台模板风格。特征是传统的“蓝白灰”配色（`#1890ff` 主题色）、明显的卡片阴影、硬朗的边框（`#f0f0f0`）以及灰色的应用背景。这种设计足够实用，但缺乏现代感和“Wow”的视觉冲击力。
- **桌面 UI 概念 (Cyberpunk / Glassmorphism)**：你在桌面文件夹中的设计采用了深色玻璃拟物化、霓虹发光（青色/紫色渐变）的风格。这种风格非常炫酷，但与 Threads 的设计理念**完全相反**。

**Threads 的设计哲学**：
Threads (Meta) 是现代极简主义的代表。它的核心特点是：
1. **极致的单色调 (Monochrome)**：摒弃复杂的渐变和鲜艳的主题色，主要依靠黑白灰的强烈对比来引导视觉。
2. **高圆角 (High Border Radius)**：药丸状 (Pill-shaped) 的按钮和高度圆润的卡片。
3. **扁平与无阴影 (Flat & No Shadows)**：完全放弃玻璃拟物化和厚重的投影，依靠极细的边框（如 `1px solid #E5E5E5`）来区分层级。
4. **内容优先 (Content-First)**：导航栏和菜单极度克制，留白非常多，让数据和文本成为页面的主角。

---

## 2. Threads 风格设计语言指南 (Design System)

要将你的 IFRS 16 管理系统改造为 Threads 风格的“现代极简”企业应用，我们需要制定以下设计令牌 (Design Tokens)：

| 元素 | Threads 风格规范 |
| :--- | :--- |
| **色彩 (Colors)** | 主色调：纯黑 `#000000` (或纯白 `#FFFFFF`)<br>背景色：纯白 `#FFFFFF` (亮色模式) 或 `#0A0A0A` (暗色模式)<br>边框色：极浅灰 `#EAEAEA` 或 `#333333` |
| **排版 (Typography)** | 字体：首选 `Inter` 或苹果系统默认字体 (San Francisco)。<br>层级：加粗的大标题，搭配极简的副标题文本。 |
| **形状 (Shapes)** | 按钮圆角：`9999px` (药丸形)<br>卡片/弹窗圆角：`16px` 或 `24px` |
| **交互 (Interaction)** | 悬浮效果极度克制，通常只改变极浅的背景色（如 hover 时背景变浅灰 `#F5F5F5`），取消位移和阴影变化。 |

---

## 3. 升级实施计划 (Implementation Plan)

### 阶段一：重构全局主题 (Ant Design ConfigProvider)
在 Next.js 的 `layout.tsx` 或客户端根组件中引入 Ant Design 的 `ConfigProvider`，全面覆盖默认的蓝色主题和方形边角，注入极简黑白配置。

### 阶段二：改造全局布局 (AppLayout 升级)
打破传统的“左黑右灰、中间套白盒”的后台布局。
1. **背景统一**：将 Header、Sider 和 Content 的背景全部统一为纯色（如纯白），消除割裂感。
2. **极简导航**：移除侧边栏右侧的粗实线，改为细线或完全取消，使用灰底作为选中状态，移除蓝色的 `activeBar`。
3. **沉浸式内容区**：取消 Content 区域的默认灰色外围，让内容直接在页面上延展，使用微小的细线边框卡片来包裹数据。

### 阶段三：组件级微调 (表格、按钮、弹窗)
- **按钮**：全面使用纯黑（或深灰）的实体按钮，以及带细边框的幽灵按钮。
- **表格**：移除表格的竖向边框，增加行高，让数据呼吸感更强。
- **输入框**：使用带有浅灰背景、无边框的输入框设计，聚焦时辅以黑色细边框。

---

## 4. 具体代码指导 (Step-by-Step Guide)

### 第一步：创建 Threads 风格的全局主题

首先，我们需要在应用中引入自定义的 Ant Design 主题。在 `web/app` 下创建一个新的客户端组件包裹器（或者直接修改现有的 `layout.tsx` / `AuthContext` 附近的逻辑）。建议创建一个 `ThemeProvider.tsx`。

```tsx
// web/app/components/ThemeProvider.tsx
"use client";

import React from "react";
import { ConfigProvider } from "antd";

export default function ThemeProvider({ children }: { children: React.ReactNode }) {
  return (
    <ConfigProvider
      theme={{
        token: {
          // 核心色彩：黑白灰极简
          colorPrimary: '#000000',      // 主色改为纯黑
          colorInfo: '#000000',
          colorSuccess: '#10B981',      // 保持功能色，但可以选柔和的
          colorWarning: '#F59E0B',
          colorError: '#EF4444',
          
          // 基础设置
          colorBgBase: '#FFFFFF',
          colorTextBase: '#000000',
          borderRadius: 12,             // 提高基础圆角
          wireframe: false,
          fontFamily: '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
          
          // 边框更轻柔
          colorBorder: '#E5E5E5',
          colorBorderSecondary: '#F0F0F0',
        },
        components: {
          Button: {
            borderRadius: 9999,         // 药丸按钮 (Threads 标志性特征)
            controlHeight: 40,          // 稍微增高按钮，提升触感
            fontWeight: 600,
            defaultBg: '#FFFFFF',
            defaultBorderColor: '#E5E5E5',
          },
          Card: {
            borderRadiusLG: 20,         // 卡片高圆角
            boxShadowTertiary: 'none',  // 移除卡片阴影，靠细边框区分
          },
          Menu: {
            itemBorderRadius: 8,
            activeBarBorderWidth: 0,    // 移除侧边栏选中的右侧蓝条
            itemSelectedBg: '#F5F5F5',  // 选中时仅改变底色为极浅灰
            itemSelectedColor: '#000000',// 选中文字为纯黑
            itemHoverBg: '#FAFAFA',
          },
          Layout: {
            bodyBg: '#FFFFFF',          // 消除原本的灰色背景
            headerBg: '#FFFFFF',
            siderBg: '#FFFFFF',
          },
          Table: {
            headerBg: '#FAFAFA',
            headerBorderRadius: 12,
            borderColor: '#F0F0F0',
          }
        }
      }}
    >
      {children}
    </ConfigProvider>
  );
}
```

在 `web/app/layout.tsx` 中使用它：
```tsx
import { AntdRegistry } from "@ant-design/nextjs-registry";
import { AuthProvider } from "./context/AuthContext";
import ThemeProvider from "./components/ThemeProvider"; // 引入
import "./globals.css";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>
        <AntdRegistry>
          <ThemeProvider>
            <AuthProvider>{children}</AuthProvider>
          </ThemeProvider>
        </AntdRegistry>
      </body>
    </html>
  );
}
```

### 第二步：改造 `AppLayout.tsx` 布局代码

修改现有的 `web/app/components/AppLayout.tsx`，使其剥离传统的“盒子感”。

```tsx
// 针对 AppLayout.tsx 的样式修改重点：

// 1. Header 修改：移除底边框的生硬感，或使其更轻盈
<Header
  style={{
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    background: "#fff",
    borderBottom: "1px solid #EAEAEA", // 更浅的边框
    padding: "0 24px",
    height: 64, // 固定高度
  }}
>
  <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
    {/* 使用黑色图标替代蓝色 */}
    <RobotOutlined style={{ fontSize: 24, color: "#000" }} />
    <span style={{ fontSize: 18, fontWeight: 700, letterSpacing: "-0.5px" }}>
      IFRS 16
    </span>
  </div>
  // ...
</Header>

// 2. Sider 修改：移除右侧边框，靠留白区分
<Sider
  width={240}
  style={{
    background: "#fff",
    borderRight: "1px solid #EAEAEA", // 可选：如果希望极致干净，甚至可以去掉这个 border
    padding: "16px 8px", // 给菜单外部增加呼吸感
  }}
>
  // ...
</Sider>

// 3. Content 修改：消除“套娃”感
<Content
  style={{
    margin: "0 auto",           // 居中对齐 (可选)
    padding: "32px 48px",       // 增加内边距
    background: "#fff",         // 保持纯白背景，不再是灰底白块
    minHeight: "calc(100vh - 64px)",
    maxWidth: 1440,             // 限制最大宽度，提升阅读体验
    width: "100%",
  }}
>
  <div style={{ maxWidth: "100%", margin: "0 auto" }}>
    {children}
  </div>
</Content>
```

### 第三步：页面级组件的最佳实践

在你后续开发 `contracts/page.tsx` 或表单页面时，遵循以下规范即可完美复刻 Threads 风格：
1. **不要滥用 Card 组件**：不要把整个页面包在一个大 Card 里。标题直接写在页面上，使用大字号加粗（如 `fontSize: 24, fontWeight: 700`）。
2. **表单设计**：输入框尽量使用 `variant="filled"`（Ant Design 5.x 支持），这会产生一个浅灰色背景且无边框的现代感输入框，极其贴合 Threads 风格。
3. **图标**：尽量使用线性图标 (Outlined)，避免使用面性图标 (Filled)，除非表示选中状态。

如果你准备好了，我们可以先从**第一步：注入 ThemeProvider 配置** 开始动手，看看页面整体气质的变化！
