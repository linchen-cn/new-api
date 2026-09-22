# 隐藏模型广场：列表性能字段与详情性能/API Tab 方案

## 摘要

在「模型广场」（前端路由 `/pricing`，目录 `features/pricing/`）中：

1. **模型列表**：隐藏卡片视图中的「延迟、吞吐、状态」三个性能字段。
2. **模型详情**：隐藏「性能（Performance）」与「API」两个 Tab 页，仅保留「概览（Overview）」。

仅做前端隐藏，不改动后端接口、性能指标采集逻辑与 i18n 词条。概览（Overview）Tab 内的性能指标条（TPS / 平均延迟 / 成功率）**不在本次需求范围内，保留不动**。

## 现状分析

### 1. 模型列表的「延迟、吞吐、状态」

- 三个字段由 [model-perf-badge.tsx](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-perf-badge.tsx) 的 `ModelPerfBadge` 组件渲染：
  - [L80-L87](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-perf-badge.tsx#L80-L87)：延迟（`Latency short` / `Average latency`）
  - [L88-L95](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-perf-badge.tsx#L88-L95)：吞吐（`Throughput short` / `Throughput`）
  - [L96-L121](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-perf-badge.tsx#L96-L121)：状态（`Status short` / 成功率竖条）
- 唯一渲染点：`model-card.tsx` L248 `<ModelPerfBadge perf={props.perf} ... />`。
- 数据来源：`model-card-grid.tsx` L47-L65 用 `getPerfMetricsSummary(24)` 拉取并构建 `perfMap` 传给 `ModelCard`。
- **表格视图**（`pricing-columns.tsx`）本来就没有这三列，无需改动。
- 引用关系（Grep 确认）：`model-perf-badge.tsx` 仅被 `model-card.tsx`（组件+类型）与 `model-card-grid.tsx`（类型 `ModelPerfBadgeData`）引用。

### 2. 模型详情的「性能、API」Tab

- 文件：[model-details.tsx](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-details.tsx)
- [L1129](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-details.tsx#L1129)：`const TAB_VALUES = ['overview', 'performance', 'api'] as const`
- [L1132-L1139](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-details.tsx#L1132-L1139)：`TAB_META` 含 `performance: { icon: HeartPulse, labelKey: 'Performance' }`、`api: { icon: Code2, labelKey: 'API' }`
- [L1165-L1180](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-details.tsx#L1165-L1180)：`TabsList` 用 `grid-cols-3` 遍历渲染 3 个 `TabsTrigger`
- [L1212-L1221](file:///Users/chenlin/go/new-api/web/default/src/features/pricing/components/model-details.tsx#L1212-L1221)：`performance`、`api` 两个 `TabsContent`
- 相关 import：
  - L25 `Code2`、L27 `HeartPulse`（lucide-react；**HeartPulse 还被 L220 的概览成功率指标用到，不能删**）
  - L77 `import { ModelDetailsApi }`、L78 `import { ModelDetailsPerformance }`
- `endpointMap` prop 仅被 api Tab 使用（L1216-L1221）；删除 api Tab 后其接口字段与调用方传参保留（兼容，避免牵扯 `usePricingData`）。

## 变更方案

### A. 模型列表：移除性能字段（延迟/吞吐/状态）

| 文件 | 改动 |
|---|---|
| `web/default/src/features/pricing/components/model-card.tsx` | 删除 L248 的 `<ModelPerfBadge ... />` 渲染；删除 L35 `import { ModelPerfBadge, type ModelPerfBadgeData }`；删除 L44 的 `perf?: ModelPerfBadgeData` prop（已无使用）。 |
| `web/default/src/features/pricing/components/model-card-grid.tsx` | 删除 L47-L52 的 `perfQuery`、L59-L65 的 `perfMap`；删除 L24 `import { getPerfMetricsSummary }`、L28 `import type { ModelPerfBadgeData }`；删除 L82 传给 `ModelCard` 的 `perf={...}`。 |
| `web/default/src/features/pricing/components/model-perf-badge.tsx` | **删除整个文件**（移除全部引用后已无使用者，符合项目「确定未使用的代码可整体删除」的规范；git 历史可随时找回）。 |

说明：
- 删除 `perf` prop 后需同步确认 `ModelCard` 其余调用点（当前仅 `model-card-grid.tsx` 一处）。
- 该文件顶部版权头（QuantumNous）随文件一并删除，不属于受保护标识改动，无影响。

### B. 模型详情：隐藏「性能」「API」两个 Tab

文件：`web/default/src/features/pricing/components/model-details.tsx`

| 位置 | 改动 |
|---|---|
| L1129 | `TAB_VALUES = ['overview'] as const`（去掉 `'performance'`、`'api'`） |
| L1132-L1139 | `TAB_META` 删除 `performance`、`api` 两个条目，仅留 `overview` |
| L1166 | `TabsList` 的 `grid-cols-3` → `grid-cols-1` |
| L1212-L1221 | 删除 `performance`、`api` 两个 `TabsContent` 块（含 `ModelDetailsPerformance`、`ModelDetailsApi` 调用） |
| L25 | 删除 lucide `Code2` import（仅 api Tab 使用） |
| L77-L78 | 删除 `import { ModelDetailsApi }`、`import { ModelDetailsPerformance }` |

保留不动：
- `HeartPulse`（L220 概览成功率指标仍在使用）
- `endpointMap` 接口字段（L1145）与调用方传参（`index.tsx` L268、`model-details.tsx` L1354）——接口保持兼容，仅不再被渲染引用。
- `OverviewSummaryGrid`（L173-L227）内 TPS / 平均延迟 / 成功率指标条。

### C. i18n

不改动。以下 key 在隐藏后可能不再被上述组件使用，但多为通用词（`API`、`Performance`、`Status` 等），其他模块可能引用，删除存在误伤风险；本次仅隐藏 UI，i18n 词条保留不影响功能：

- `Latency short` / `Throughput short` / `Status short` / `Average latency` / `Throughput` / `Success rate` / `Performance` / `API` / `Overview`

### D. 验证步骤

1. `bunx tsc --noEmit -p tsconfig.json`（web/default 目录）无新增类型错误。
2. `bunx oxlint`（web/default）无新增违规（重点确认无未使用 import）。
3. `bun run build`（web/default）通过。
4. 手工验收：
   - 访问 `/pricing`：卡片视图中每张模型卡不再显示「延迟 / 吞吐 / 状态」三字段，卡片布局正常。
   - 点击任意模型进入详情（抽屉与独立页 `/pricing/$modelId` 共用同一 `ModelDetailsContent`，两处均生效）：仅剩「概览」一个 Tab，无「性能」「API」Tab；概览内仍显示定价、分组定价与 TPS/延迟/成功率指标条。

## 假设与决策

1. 本次只做**前端隐藏**，不动后端 `/api/...perf...` 采集接口与数据存储。
2. 只隐藏**卡片视图**的三字段（表格视图本无这三列）。
3. 详情页仅隐藏两个 Tab，**概览页内的性能指标条保留**（用户未要求隐藏）。
4. `model-perf-badge.tsx` 整体删除（无引用后不留死代码）；如后续需恢复可经 git 找回或加回引用。
5. `endpointMap` prop 保留但不再渲染使用，避免牵连 `usePricingData` 与调用方（改动最小化）。
6. i18n 词条不清理，规避误伤其他模块。
