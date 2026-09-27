# 后付费客户对账单系统(Billing Statements)实施方案

## Summary

为"后付费客户"建立完整的对账单体系:

1. **后付费客户体系**:User 新增 `billing_type`(prepaid/postpaid)与 `credit_limit`(信用额度)字段,管理端可编辑、可筛选
2. **实时信用控制**:后付费用户 quota 允许为负,欠费超过信用额度时自动拒绝新请求(改 relay 预扣费校验,共 2 处)
3. **正式账单实体**:`billing_statements` + `statement_adjustments` 两张新表,出账时固化快照(不受 `DeleteOldLog` 日志清理影响),状态流转 unpaid → paid / voided
4. **账单粒度**:总额汇总(消费/退款/调整/应收)+ 按模型分组明细
5. **调整行**:出账后可添加折扣/减免(负)/罚金(正),实时重算应收
6. **结清机制**:标记"已付"时可同时把账单欠款补回用户余额(事务内 quota += TotalQuota)
7. **管理后台**:新增"对账单"页面(预览→出账→详情→调整→收款→导出 CSV),用户管理页支持后付费编辑

金额口径:全部存 quota(整数),展示按 `QuotaPerUnit`(1 美元 = 500,000 quota)换算,货币符号跟随系统现有显示配置。

---

## Current State Analysis(Phase 1 探索结论)

### 已具备(无需新建)

| 数据/能力 | 位置 |
| --- | --- |
| 消费明细(logs, type=2)含 user/time/model/quota/tokens/group | [model/log.go:34-68](file:///Users/chenlin/go/new-api/model/log.go#L34-L68) |
| 日志类型常量 Consume=2 / Topup=1 / Refund=6 | [model/log.go:70-80](file:///Users/chenlin/go/new-api/model/log.go#L70-L80) |
| 按用户+时间范围消费汇总(单值) | `SumUsedQuota`, model/log.go:596 |
| 充值流水表 topups(Money 为美元) | [model/topup.go:14-25](file:///Users/chenlin/go/new-api/model/topup.go#L14-L25) |
| 日志库分离分支模式 `common.UsingLogDatabase(...)` | model/log.go 各查询函数 |
| User 表 + 余额函数 | [model/user.go:24-57](file:///Users/chenlin/go/new-api/model/user.go#L24-L57) |
| AutoMigrate 注册点 | [model/main.go:271](file:///Users/chenlin/go/new-api/model/main.go#L271) |
| 预扣费校验点(2 处) | [service/pre_consume_quota.go:38-43](file:///Users/chenlin/go/new-api/service/pre_consume_quota.go#L38-L43)、[service/billing_session.go:390-397](file:///Users/chenlin/go/new-api/service/billing_session.go#L390-L397) |
| 管理路由分组模式(AdminAuth) | [router/api-router.go](file:///Users/chenlin/go/new-api/router/api-router.go)(/api/log 段参照) |
| 用户编辑抽屉 + zod schema | [users-mutate-drawer.tsx](file:///Users/chenlin/go/new-api/web/default/src/features/users/components/users-mutate-drawer.tsx)、[user-form.ts](file:///Users/chenlin/go/new-api/web/default/src/features/users/lib/user-form.ts)(remark 是现成参照) |
| 前端 feature 目录约定(api.ts/types.ts/components/) | web/default/src/features/* |

### 缺口(全部由本方案新建)

- 后付费概念(全库无 postpaid 标记) → User 加字段
- 账单实体/账期/状态 → 两张新表
- 分组聚合 SQL → 新增 model 层函数
- 聚合/出账/调整/结清 API → 新增 controller
- CSV 导出(前后端均无) → 新增
- 管理页面 → 新增 feature + 路由 + 侧边栏菜单

### 风险点(设计已规避)

1. **日志清理**:出账时把汇总+分组快照存进账单行,出账后不依赖 logs
2. **三库兼容**:新表走 GORM AutoMigrate(新表无 SQLite ADD COLUMN 硬编码清单坑,任务 A 的坑是 subscription 专属 ensure 函数);聚合 SQL 用纯标准 `SUM/COUNT/GROUP BY`(无方言函数);JSON 分组明细存 `TEXT` 列(不用 DB JSON 类型)
3. **热路径性能**:信用控制需要 billing_type/credit_limit。`GetUserQuota` 本身每请求一次查询,新增字段从同一条用户查询取出,不增加额外查询次数

---

## Proposed Changes

### Phase A — 后端数据模型与迁移

**新建 `model/billing_statement.go`**:

```go
type BillingStatement struct {
    Id             int    `json:"id"`
    StatementNo    string `json:"statement_no" gorm:"unique;type:varchar(64);index"` // ST-202609-{userId}-{8位随机}
    UserId         int    `json:"user_id" gorm:"index"`
    Username       string `json:"username" gorm:"index;type:varchar(64)"`
    PeriodStart    int64  `json:"period_start"` // 秒级时间戳,含
    PeriodEnd      int64  `json:"period_end"`   // 秒级时间戳,不含(便于连续账期)
    ConsumeQuota   int64  `json:"consume_quota"` // 消费总额(logs type=2 SUM)
    RefundQuota    int64  `json:"refund_quota"`  // 退款总额(logs type=6 SUM,正数)
    TopupQuota     int64  `json:"topup_quota"`   // 周期内充值总额(仅参考展示,不计入应收)
    AdjustQuota    int64  `json:"adjust_quota"`  // 调整行合计(可负)
    TotalQuota     int64  `json:"total_quota"`   // 应收 = ConsumeQuota - RefundQuota + AdjustQuota
    RequestCount   int64  `json:"request_count"`
    PromptTokens   int64  `json:"prompt_tokens"`
    CompletionTokens int64 `json:"completion_tokens"`
    ModelBreakdown string `json:"model_breakdown" gorm:"type:text"` // JSON 数组快照,见下
    Status         string `json:"status" gorm:"type:varchar(16);index;default:'unpaid'"` // unpaid/paid/voided
    SettledQuota   int64  `json:"settled_quota"` // 标记已付时结转的额度(勾选结清时 = TotalQuota,未勾选 = 0)
    PaidAt         int64  `json:"paid_at"`
    PaidNote       string `json:"paid_note"`
    Remark         string `json:"remark"`
    CreatedAt      int64  `json:"created_at" gorm:"index"`
    UpdatedAt      int64  `json:"updated_at"`
}

type StatementAdjustment struct {
    Id          int    `json:"id"`
    StatementId int    `json:"statement_id" gorm:"index"`
    Amount      int64  `json:"amount"` // quota,负=减免/折扣,正=加价/罚金
    Reason      string `json:"reason"`
    CreatedAt   int64  `json:"created_at"`
    CreatedBy   string `json:"created_by"` // 操作管理员
}
```

`ModelBreakdown` JSON 结构(出账时固化,格式用 `common.Marshal`):

```json
[{"model":"gpt-4o","quota":123000,"requests":42,"prompt_tokens":1000,"completion_tokens":2000}, ...]
```

**修改 `model/user.go`** — User 结构体新增:

```go
BillingType string `json:"billing_type" gorm:"type:varchar(16);default:'prepaid'"` // prepaid / postpaid
CreditLimit int64  `json:"credit_limit" gorm:"type:bigint;default:0"`            // 信用额度(quota)
```

**修改 `model/main.go`**:
- line 271 `DB.AutoMigrate(...)` 列表加入 `&BillingStatement{}`、`&StatementAdjustment{}`
- line 326 附近的模型清单数组同样加入(保持两处一致)
- User 新列由 AutoMigrate 自动 ADD COLUMN,三库均兼容

**修改 `controller/user.go`** — UpdateUser/EditUser 接受 `billing_type` 与 `credit_limit`(admin-only 字段,与 remark 同模式);用户查询响应自然带出。校验:`billing_type ∈ {prepaid, postpaid}`、`credit_limit >= 0`。

### Phase B — 实时信用额度控制(relay 热路径)

**新建 `model/user.go` 辅助函数**(或放同文件,就近):

```go
// GetUserQuotaAndCreditFloor 一次查询取余额与信用下限
// postpaid: 返回 -CreditLimit;prepaid: 返回 0
func GetUserQuotaAndCreditFloor(userId int) (quota int, creditFloor int64, err error)
```

实现:`SELECT quota, billing_type, credit_limit FROM users WHERE id = ?`,转换为下限。

**修改 `service/pre_consume_quota.go:33-43`** `PreConsumeQuota`:

```go
userQuota, creditFloor, err := model.GetUserQuotaAndCreditFloor(relayInfo.UserId)
...
if int64(userQuota) < creditFloor {
    return ...("用户额度不足, 剩余额度: %s, 信用额度已超限" ...)  // 保持 ErrorCodeInsufficientUserQuota + 403
}
if int64(userQuota)-int64(preConsumedQuota) < creditFloor {
    return ...("预扣费额度失败 ...")  // 同上
}
```

**修改 `service/billing_session.go:390-397`**(钱包路径的同款判断)同样改用 creditFloor。

注意:其余逻辑(trustQuota、订阅回退)不动;预付费用户 creditFloor=0,行为与现在完全一致。

### Phase C — 聚合、出账、调整、结清(model + controller)

**新建 `model/billing_statement.go` 聚合函数**(全部走 `common.UsingLogDatabase` 分支,参照 `SumUsedQuota` 模式):

```go
type StatementAggregation struct {
    ConsumeQuota     int64
    RefundQuota      int64
    RequestCount     int64
    PromptTokens     int64
    CompletionTokens int64
    ModelRows        []StatementModelRow
}

// 按模型分组聚合:logs WHERE user_id=? AND type=LogTypeConsume AND created_at>=? AND created_at<?
// SELECT model_name, SUM(quota), COUNT(*), SUM(prompt_tokens), SUM(completion_tokens) GROUP BY model_name
func AggregateUserUsage(userId int, start, end int64) (*StatementAggregation, error)
// 退款: type=LogTypeRefund SUM(quota)
// 周期内充值: topups 表 SUM(amount) WHERE user_id=? AND complete_time BETWEEN ? AND ? AND status='success'
```

**CRUD/事务函数**(同文件):
- `CreateBillingStatement(...)` — 出账:生成 StatementNo(格式 `ST-{periodStart YYYYMM}-{userId}-{8位hex随机}`),聚合一次,快照落库。**重叠校验**:同 user 已存在非 voided 账单且 `(start < s.PeriodEnd && end > s.PeriodStart)` → 返回冲突错误(携带已存在账单号)
- `AddStatementAdjustment(...)` — 插入调整行 + 重算 `AdjustQuota`/`TotalQuota` + UpdatedAt(单事务)
- `DeleteStatementAdjustment(...)` — 删除调整行 + 同上重算(单事务)
- `MarkStatementPaid(stmtId, settle bool, note)` — 事务内:status unpaid→paid、写 PaidAt/PaidNote/SettledQuota;`settle=true` 时 `IncreaseUserQuota(userId, TotalQuota)`(把账单欠款补回余额)并记 `LogTypeManage` 日志("账单 ST-xxx 已收款,结转欠费 X");paid 状态不可再改
- `VoidStatement(stmtId)` — unpaid→voided(已付账单不可作废;作废不返还 quota)
- `GetBillingStatementsPage(...)`, `GetBillingStatementById(...)`, `GetPostpaidUsers(...)`(billing_type=postpaid 用户 + 当前余额(负=欠费)+ 未结清账单数与合计)

**新建 `controller/billing_statement.go`** — 9 个 handler,绑定路由:

```
GET  /api/billing/postpaid-users                      → 后付费客户列表(下拉数据源)
GET  /api/billing/statement/preview                   → ?user_id=&start=&end= 预览聚合(不落库),附带该用户当前余额
POST /api/billing/statement/                           → body {user_id, period_start, period_end, remark} 出账
GET  /api/billing/statement/                           → 列表 ?p=&page_size=&user_id=&status=&start=&end=
GET  /api/billing/statement/:id                        → 详情(含调整行)
PUT  /api/billing/statement/:id/status                 → body {status:"paid"|"voided", settle:bool, note}
POST /api/billing/statement/:id/adjustment            → body {amount, reason}
DELETE /api/billing/statement/:id/adjustment/:adjId
GET  /api/billing/statement/:id/export                 → CSV 下载(见 Phase D)
```

**修改 `router/api-router.go`** — 在 `/api/log` 路由组附近新增:

```go
billingRoute := apiRoute.Group("/billing")
billingRoute.Use(middleware.AdminAuth())
{ ... 上述 9 条 ... }
```

**修改 `controller/user.go`** — 用户列表/搜索 API 支持按 `billing_type` 过滤(后台筛选后付费客户)。

### Phase D — CSV 导出

`controller/billing_statement.go` 内 `ExportBillingStatementCsv`:
- `Content-Type: text/csv` + `Content-Disposition: attachment; filename="statement-{statement_no}.csv"`
- **UTF-8 with BOM**(Excel 中文兼容)
- 内容三段:①账单头(账单号/用户/账期/状态/各汇总项/应收)②按模型分组明细(模型/金额/请求数/输入 tokens/输出 tokens)③调整行(原因/金额)
- 金额列输出两位小数(`quota/QuotaPerUnit`),同时附 quota 原始整数列
- 分隔符:`,`;若字段含逗号/引号按 RFC 4180 转义

### Phase E — 前端:用户管理增强

- **[user-form.ts](file:///Users/chenlin/go/new-api/web/default/src/features/users/lib/user-form.ts)**:zod schema 加 `billing_type: z.enum(['prepaid','postpaid']).default('prepaid')`、`credit_limit: z.number().int().min(0).default(0)`;默认值、提交 payload 参照 remark 模式
- **[users-mutate-drawer.tsx](file:///Users/chenlin/go/new-api/web/default/src/features/users/components/users-mutate-drawer.tsx)**:remark 字段块附近加 Select(预付费/后付费)+ 数字输入"信用额度(美元)"(仅 billing_type=postpaid 时显示;输入美元,提交前 × QuotaPerUnit 换 quota,展示时反向换算,与现有钱包页金额换算工具一致)
- **[users-columns.tsx](file:///Users/chenlin/go/new-api/web/default/src/features/users/components/users-columns.tsx)**:用户名列加小徽章"后付费"(billing_type=postpaid 时),便于后台识别
- **[types.ts](file:///Users/chenlin/go/new-api/web/default/src/features/users/types.ts)**:User schema 加两字段

### Phase F — 前端:对账单管理页

**新建 `web/default/src/features/billing-statements/`**(遵循 feature 目录约定):

```
api.ts        — 9 个接口封装 + 请求/响应类型
types.ts      — BillingStatement / StatementAdjustment / PostpaidUser / StatementPreview
index.tsx     — 页面:顶部筛选(用户名/状态/时间范围)+ 数据表格
  列:账单号、用户、账期、消费、调整、应收(美元)、状态徽章、操作(详情/已付/作废/导出)
  工具栏:"生成账单"按钮(打开 create dialog)
components/statement-create-dialog.tsx
  — 步骤式:① 选后付费用户(下拉,显示当前余额)② 时间范围(DateRange,默认上个自然月)③
    调 preview API 展示汇总卡片(消费/退款/充值参考/笔数/tokens)+ 按模型分组预览表 ④ 填备注 → "确认出账"
  — 后端冲突(账期重叠)时 toast 展示已存在账单号
components/statement-detail-sheet.tsx
  — 侧滑详情:汇总卡片、按模型分组表(只读快照)、调整行列表(可增删,行内表单:金额±、原因)
    底部操作区:"标记已付"(弹小确认:是否同时结清欠费余额(默认勾)+ 收款备注)、"作废"
```

**路由注册**:
- `web/default/src/routes/` 新建 `billing-statements` 路由文件(参照 usage-logs 的 route 文件复制模式),路径 `/console/billing-statements`(以现有 logs 路由实际路径风格为准,执行时对齐)
- 侧边栏菜单(admin 可见区)加"对账单"项:找到现有 admin 侧边栏数据文件执行时对齐,参照"日志"菜单项模式

### Phase G — i18n 与验证

- 新增 key 走英文源串为 key 的惯例(如 `Generate statement` / `Mark as paid` / `Credit limit` / `Billing type` / `Postpaid` / `Statement No.` / `Adjustment` / `Settle balance` 等),`bun run i18n:sync` 后补齐 zh/en/fr/ru/ja/vi 翻译
- 后端 i18n:错误信息沿用现有中文硬编码格式(与 relay 现有报错风格一致,如"用户额度不足")

---

## Assumptions & Decisions

1. **金额单位**:库内一律 quota(int64),展示层除以 `QuotaPerUnit`(500,000/美元);信用额度输入/展示用美元,存储换 quota
2. **应收口径**:`TotalQuota = Consume - Refund + Adjust`。周期内充值(TopupQuota)只作参考展示,不抵扣应收(后付费客户充值即打款,直接找零到余额,不参与账单抵扣;如需抵扣可后续作为调整行手工录入)
3. **计费来源不拆分**:账单不区分订阅/钱包消费(`billing_source` 在 Other JSON 里,跨库无法 SQL 聚合);后付费客户通常无订阅,若混用则全口径计入消费
4. **出账不冻结使用**:出账后客户继续调用,欠款自然累积到下期;账单快照只代表账期窗口
5. **结转语义**:标记已付时勾选"结清欠费"→ 事务内 `quota += TotalQuota`;不勾选仅记录收款。作废不返还任何额度
6. **状态机**:unpaid → paid / voided,均为单向;paid/voided 终态;调整行仅 unpaid 状态可增删
7. **防重**:同一用户非作废账单的账期重叠即拒绝出账(错误中返回已有账单号)
8. **手动出账**(用户已选):无定时任务;账期默认建议上个自然月,但允许任意范围
9. **权限**:全部账单 API 套 `AdminAuth`;billing_type/credit_limit 编辑仅管理端,用户自助接口不暴露
10. **日志清理规避**:出账即固化 ModelBreakdown 快照,历史 logs 被清理不影响已出账单
11. **热路径零额外开销**:`GetUserQuota` 改为一次查询同时取 quota+billing_type+credit_limit,不增加查询次数

## Files Touched(汇总)

| 文件 | 性质 |
| --- | --- |
| model/billing_statement.go | 新 — 实体/聚合/CRUD/事务 |
| model/user.go | 改 — User +2 字段;新查询函数 |
| model/main.go | 改 — AutoMigrate 注册新表 |
| service/pre_consume_quota.go | 改 — 信用下限校验 |
| service/billing_session.go | 改 — 钱包路径同款校验 |
| controller/billing_statement.go | 新 — 9 个 handler + CSV |
| controller/user.go | 改 — 编辑/筛选 billing_type |
| router/api-router.go | 改 — /api/billing 路由组 |
| web/default/src/features/billing-statements/* | 新 — 页面/组件/api/types |
| web/default/src/routes/billing-statements*(命名对齐现有惯例) | 新 — 前端路由 |
| users-mutate-drawer.tsx / user-form.ts / users-columns.tsx / types.ts | 改 — 后付费编辑与徽章 |
| 侧边栏菜单数据文件 | 改 — 加"对账单"入口 |
| web/default/src/i18n/locales/*.json | 改 — 新 keys |

## Verification

1. **后端构建**:`go build ./...`
2. **后端单测**(新建 `model/billing_statement_test.go`,SQLite 内存库,`testify require/assert`,表驱动):
   - 聚合正确性:构造 consume/refund/topup 样本 → `AggregateUserUsage` 各分项与按模型分组精确匹配
   - 账期边界:含 start 不含 end;跨重叠校验拒绝出账并返回既有账单号
   - 状态机:paid 后不可再改/不可作废;voided 后不可调整
   - 结转事务:MarkStatementPaid(settle=true) 后用户 quota 增加恰为 TotalQuota;settle=false 不变
   - 调整行:增/删后 AdjustQuota/TotalQuota 重算正确
   - 信用下限:prepaid floor=0(现行为不变,回归);postpaid 且 quota==-limit 通过、quota==-limit-1 拒绝
3. **回归**:`go test ./service/ ./model/ ./pkg/...`(确保 relay 计费路径无回归)
4. **前端**:`bun run typecheck`(排除已知历史存量错误)/ `bun run lint` / `bun run build`
5. **i18n**:`bun run i18n:sync` 无缺 key
6. **手工冒烟**:
   - 用户管理:把测试用户设为后付费+信用额度 $10 → 切到该用户调用,quota 扣为负仍可调用;欠超 $10 后调用返回 403"信用额度已超限"
   - 出账:选该用户+上月范围 → 预览数字与日志页人工核对一致 → 出账 → 账单列表出现;重叠再出账被拒
   - 调整:加 -5% 折扣行,应收变小;删除恢复
   - 收款:标记已付(勾结清)→ 用户余额回正、状态徽章变已付、PaidAt 落值
   - 导出:CSV 打开无乱码、金额与页面一致
   - 日志清理设置开启后模拟清理,已出账单详情数据完好

## Out of Scope(本期不做)

- 自动出账定时任务(用户已选手动)
- PDF 账单排版(CSV 先行)
- 账单邮件发送/客户自助查看账单门户
- 信用额度按账期动态调整、账期逾期利息
- 订阅消费在账单中单列拆分(billing_source JSON 无法跨库 SQL 聚合)
