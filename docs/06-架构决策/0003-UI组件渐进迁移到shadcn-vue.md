---
status: accepted
---

# UI 组件渐进迁移到 shadcn-vue

## 背景

现有前端是 Vue 3 + TDesign Vue Next + Less。用户要求后续 UI 设计从 `shadcn.com.cn` 获取组件方式，同时保持当前 WellRead 配色。`shadcn.com.cn` 的组件实现是 React + Tailwind，不能直接复制到 Vue 项目；其 Vue 对应实现是社区维护的 `shadcn-vue`。

## 决策

1. 组件模式参考 `shadcn.com.cn`，Vue 实现使用 `shadcn-vue` + Tailwind CSS v4。
2. 首轮只迁移复习页；其他页面继续使用 TDesign，禁止在同一页面混用两套组件。
3. shadcn-vue 组件源码进入 `src/components/ui`，按需添加，不引入全量组件库。
4. 保持现有 WellRead 视觉语言；shadcn 语义 token 映射现有 TDesign CSS 变量，品牌绿、浅色容器与 8px 圆角不变。
5. 首版不全局引入 Tailwind Preflight，避免重置 TDesign 旧页面；组件基础重置局部化。全前端迁移完成后再评估全局重置。
6. 统计页图表使用 ECharts，不依赖 shadcn-vue Chart；图表颜色同样映射品牌 token。
7. 全前端迁移列为后续独立批次，不在复习 V2 中扩散。

## 考虑过的替代方案

- **直接复制 shadcn.com.cn 的 React 源码**：技术上不可用，React、Radix 与 Vue 运行时不同。
- **不引入新组件体系，仅按 shadcn 视觉用 TDesign 重做**：改动最小、风格统一，但无法获得 shadcn 的组件源码、组合方式与可访问性模式。
- **立刻全前端迁移**：长期目标一致，但会同时触碰所有页面，风险与回归范围过大，无法先验证复习页效果。
- **启用 Tailwind 全局 Preflight**：组件默认样式更完整，但会重置现有 TDesign 页面，违反“保持当前风格不变”。

## 后果

- 前端短期存在 TDesign 与 shadcn-vue 双栈；必须用页面边界隔离，不允许同页混用。
- 需要维护 `components.json`、Tailwind 入口、主题 token 映射与深色模式桥接。
- 新增 shadcn-vue 相关依赖与少量包体；ECharts 也增加包体，需按需引入并观察构建体积。
- 复习 V2 必须先完成组件冒烟、浅色/深色、窄屏与键盘验收，再扩大迁移范围。
- 文档入口：`docs/02-需求与设计/界面规范.md` §6、`frontend/README.md`、`CLAUDE.md` §3。
