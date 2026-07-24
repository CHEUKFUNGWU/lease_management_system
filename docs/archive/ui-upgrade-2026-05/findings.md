# Findings & Decisions — IFRS 16 UI/UX 升级

## Requirements
- 将现有 MVP 级 UI 升级至专业企业 SaaS 水准
- 提升视觉一致性、信息层级、交互反馈和响应式体验
- 保持现有 Next.js 14 + Ant Design + Tailwind 技术栈
- 支持中/英/繁三种语言

## Research Findings

### 现状代码审查结果（基于 page.tsx, AppLayout.tsx, globals.css, contracts/page.tsx）

#### 1. 色彩系统
- **现状**：全局仅使用黑白灰（#000, #fff, #666, #999, #EAEAEA）+ Ant Design 默认状态色
- **问题**：
  - 缺少品牌主色，视觉无记忆点
  - 状态色（成功/警告/错误）未统一覆盖，部分仍使用 Ant Design 默认蓝/橙/红
  - 没有语义化的色彩层级（背景层、表面层、强调层）
  - 不支持暗色模式

#### 2. 排版系统
- **现状**：使用 Inter 字体，h1/h2 有基础样式覆盖
- **问题**：
  - 标题字号偏小（h1=24px），在企业级大屏上显得单薄
  - 缺少数据展示专用字体样式（金额、百分比、日期）
  - 行高未标准化，中文阅读体验可以优化
  - 未针对中文做排版微调（如适当增大字间距、行高）

#### 3. 布局与导航
- **现状**：经典 Header + Sider + Content 三栏布局
- **问题**：
  - Sider 宽度 220px，在大屏（2K+）上显得窄小
  - 缺少面包屑导航，深层页面返回成本高
  - 没有标签页记忆（Tab Persistence），多任务切换不便
  - FloatButton 全局存在，但位置固定，可能遮挡内容
  - 菜单图标风格不统一（部分使用 outlined，部分未明确）

#### 4. 组件一致性
- **现状**：Ant Design 组件 + 部分全局样式覆盖
- **问题**：
  - Card 强制无边框阴影，但在白色背景上缺乏边界感
  - 表格表头文字颜色 #666 在暗色背景下未适配
  - 按钮样式覆盖不完整（仅处理了 primary hover）
  - Modal 圆角 20px 与其他组件（如 Card 默认圆角）不协调
  - Input focus ring 使用黑色，在蓝色主色调场景下突兀

#### 5. 数据展示
- **现状**：Dashboard 使用 Statistic + 简单列表
- **问题**：
  - 无图表/可视化，数据趋势不可见
  - 金额展示无千分位/货币符号格式化
  - 表格列宽未优化，长文本截断策略不明确
  - 空状态（Empty State）使用默认样式，缺乏引导性

#### 6. 交互反馈
- **现状**：基础 hover 过渡（0.15s）
- **问题**：
  - 页面切换无过渡动画，体验生硬
  - 数据加载使用 Spin，缺少骨架屏（Skeleton）
  - 表单提交无进度/状态反馈
  - 审批流状态变化无视觉动效
  - 缺少操作成功/失败的 Toast 动画细节

#### 7. 响应式
- **现状**：基础断点（xs/sm/lg）+ 简单 grid 调整
- **问题**：
  - 未针对平板（768px-1024px）做专门优化
  - Sider 在小屏下无收起/抽屉逻辑
  - 表格在移动端横向滚动体验差
  - 复杂表单的布局在小屏下堆叠混乱

## Technical Decisions（更新）
| Decision | Rationale |
|----------|-----------|
| 使用 Ant Design ConfigProvider 定制主题 | 官方推荐方式，可以系统级覆盖组件样式，比 globals.css 更可控 |
| Tailwind + CSS Variables 双轨管理 | Tailwind 负责布局工具类，CSS Variables 负责主题色值，各司其职 |
| 引入 framer-motion 处理动画 | React 生态最佳动画库，声明式 API，与 Next.js App Router 兼容 |
| 引入 recharts 处理图表 | 轻量、React 原生、可完全定制为黑白灰极简风格 |
| 建立 `web/app/design-system/` 目录 | 集中管理 tokens、theme、constants，避免分散在各组件中 |
| **无暗色模式** | 用户明确不需要，专注打磨单一亮色主题到极致 |
| **无移动端适配** | 用户明确不需要，focus 在桌面端体验（1920/1440/1280） |
| **拒绝厚重阴影和渐变** | 极简主义核心原则。深度仅通过 1-2px blur 的极淡阴影和精细边框层次表达 |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| Ant Design 5.x 默认 token 与 Tailwind 冲突 | 计划通过 ConfigProvider theme 完全覆盖，禁用 Ant Design 默认色彩 |

## Resources
- Ant Design 5 Theme 配置文档：https://ant.design/docs/react/customize-theme
- Tailwind CSS 配置：https://tailwindcss.com/docs/configuration
- Framer Motion：https://www.framer.com/motion/
- Recharts：https://recharts.org/

## Visual/Browser Findings
- 当前 Dashboard 在 1440px 下左右留白不均（Content maxWidth=1440 但无居中约束）
- AppLayout Header 使用 sticky，但 zIndex=100 可能与其他浮层冲突
- 合同列表表格操作列宽度不固定，导致列跳动
- 登录页未在本次审查范围内，需后续补充
- **关键观察**：当前系统已具备黑白灰基础，但缺乏"层次"——所有元素都是纯黑或纯白，缺少中间灰阶来构建视觉深度
