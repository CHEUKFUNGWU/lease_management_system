# Task Plan: IFRS 16 Web UI/UX 全面升级（黑白灰极简主义）

## Goal
在黑白灰极简主义框架下，将 IFRS 16 租赁管理系统的 Web 前端 UI/UX 升级至专业级企业 SaaS 标准。通过极致的排版层次、材质感、微交互和动效，让极简不单调。保持现有 Ant Design + Tailwind 技术栈，仅针对桌面端优化。

## Current Phase
Phase 2

## Phases

### Phase 1: 需求梳理与现状诊断
- [x] 审查现有代码结构（Dashboard, Layout, Contracts, globals.css）
- [x] 委托设计师进行专业 UI/UX 审计
- [x] 汇总发现，建立问题清单与优先级矩阵
- [x] 输出 UI/UX 现状诊断报告
- **Status:** complete

### Phase 2: 黑白灰设计系统深化
- [ ] 定义中性色板层次（从 #000 到 #fff 的 12 级灰阶，用于背景/表面/边框/文字）
- [ ] 定义排版层级（标题、正文、辅助文字、数据展示——在单色下排版是唯一层次工具）
- [ ] 定义间距系统（8px 基础网格 + 4px 微调）
- [ ] 定义深度系统（微妙的边框 vs 极淡的阴影，拒绝厚重投影）
- [ ] 定义微交互规范（悬浮、点击、状态切换的时序和曲线）
- [ ] 更新 globals.css 为完整的黑白灰 Design Tokens
- [ ] 通过 Ant Design ConfigProvider 统一组件样式
- **Status:** in_progress

### Phase 3: 全局组件与交互升级
- [ ] 重构 AppLayout（面包屑导航、标签页记忆、更精致的 Sider）
- [ ] 优化 Header（全局搜索、通知中心、快捷操作、更紧凑的用户菜单）
- [ ] 升级表格组件（列宽记忆、更好的空状态、行悬浮效果、选中态）
- [ ] 升级表单组件（分段表单、验证反馈动效、自动保存草稿提示）
- [ ] 添加全局过渡动画（页面切换、数据加载骨架屏、模态框进出）
- [ ] 添加微交互系统（按钮点击涟漪、卡片悬浮抬升、Tag 状态变化、审批流节点动效）
- **Status:** pending

### Phase 4: 核心页面升级
- [ ] 仪表板（Dashboard）- Recharts 极简图表（黑白灰）、KPI 卡片材质感升级、快捷操作区
- [ ] 合同台账（Contracts List）- 高级筛选面板、批量操作、视图切换（列表/紧凑列表）
- [ ] 合同详情（Contract Detail）- 信息架构重组、审批流可视化时间线、标签页平滑切换
- [ ] AI 聊天（AI Chat）- 更精致的对话界面、引用来源展示、打字机动效
- [ ] 月结跑批（Monthly Closing）- 进度指示器、批次卡片、状态时间线
- [ ] 报表（Reports）- 极简图表集成、导出预览、双模式切换优化
- [ ] 登录页（Login）- 黑白灰品牌感、安全提示
- **Status:** pending

### Phase 5: 可访问性与性能优化
- **Status:** skipped（用户确认不做）

### Phase 6: 验证与交付
- [x] TypeScript 编译验证（零错误）
- [x] 代码审查：移除所有硬编码颜色，统一黑白灰体系
- [x] 跨页面一致性检查（标题/动画/卡片/表格）
- [ ] 用户走查（关键流程：登录→合同列表→详情→审批）
- [ ] 文档更新（UI 规范文档、组件使用指南）
- [x] 交付升级报告
- **Status:** in_progress

## Key Questions（已回答）
1. 色彩方向：黑白灰极简主义，深化质感而非添加颜色 ✅
2. 暗色模式：不需要 ❌
3. 移动端：不需要，专注桌面端 ❌
4. 品牌 VI：暂无，以纯黑白灰为品牌语言
5. 用户偏好：Phase 4 中考虑最近访问、收藏等快捷功能

## Decisions Made
| Decision | Rationale |
|----------|-----------|
| 保持 Ant Design + Tailwind 栈 | 现有技术栈已足够成熟，重写成本过高；通过深度定制 Ant Design token 和 Tailwind 配置即可达到目标 |
| 黑白灰极简主义深化 | 用户明确偏好。在单色下通过材质（边框/背景层次）、排版、间距、动效来创造丰富体验 |
| 仅针对桌面端（1920/1440/1280） | 用户明确无移动端需求。 focus 在主流桌面分辨率 |
| 引入 Recharts 作为图表库 | React 生态友好、体积小、可完全定制为黑白灰风格 |
| 引入 Framer Motion 处理动画 | 声明式 API，与 Next.js App Router 兼容，适合极简风格的精致微交互 |
| 使用 CSS Variables 管理 Design Tokens | 支持运行时微调，性能开销小，与 Tailwind 无缝结合 |
| 拒绝厚重阴影和渐变 | 极简主义原则。深度仅通过极淡阴影（1-2px blur）和边框层次表达 |

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
|       | 1       |            |

## Notes
- 更新 phase 状态以跟踪进度：pending → in_progress → complete
- 每完成一个 Phase 后重新读取本计划，确保方向一致
- 所有视觉变更必须记录到 findings.md 的设计决策部分
- 避免引入破坏性变更：保持现有 API 和数据结构不变
- **核心设计原则**：在黑白灰框架下，"少即是多"——每一个像素、每一毫秒动画都必须有目的
