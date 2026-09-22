# 订阅套餐三档限额（会话/周/月）实施方案

## 一、目标与语义（已与用户确认）

仿照火山方舟 Coding Plan 的额度机制，为订阅套餐增加"三档限额"模式：

* **三档独立限额**，计量单位均为**点数（quota，按实际 token 用量计费）**：

  1. **会话档**：以首次请求时刻起算的**锚定窗口**（默认 5 小时），窗口到期清零重新计数
  2. **周档**：每周一 00:00（服务器时区）重置
  3. **月档**：自然月 1 日 00:00 重置（沿用现有 monthly 重置对齐逻辑），月度总额 = 套餐的 `TotalAmount`（即"固定总点数"）

* **任一档耗尽 → 直接限流拒绝**：返回 HTTP 429 + 提示该档恢复时间（如「会话额度已用尽，约 X 后恢复」），**不回退钱包余额**（与方舟"不消耗账户余额"一致）

* 现有单窗口套餐（never/daily/weekly/monthly/custom 重置）行为完全不变，三档模式是套餐级的新开关

## 二、现状分析（Phase 1 探索结论）

| 模块             | 位置                                                                                                 | 现状                                                                                                                                                                                       |
| -------------- | -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 套餐模型           | [model/subscription.go](../../go/new-api/model/subscription.go)                                    | `SubscriptionPlan`：`TotalAmount` + 单一 `QuotaResetPeriod`；`UserSubscription`：`AmountTotal/AmountUsed/LastResetTime/NextResetTime` 单窗口                                                     |
| 预扣费            | `model.PreConsumeUserSubscription`                                                                 | 单事务 FOR UPDATE 锁订阅行 → 窗口惰性重置（`maybeResetUserSubscriptionWithPlanTx`）→ 校验月度余量 → 写 `SubscriptionPreConsumeRecord`（requestId 幂等）→ `AmountUsed += amount`                                    |
| 结算/退款          | `PostConsumeUserSubscriptionDelta` / `RefundSubscriptionPreConsume`                                | 按 delta 调整月度；退款按 record 幂等                                                                                                                                                               |
| 资金源            | [service/funding\_source.go](../../go/new-api/service/funding_source.go)                           | `SubscriptionFunding` 实现 `PreConsume/Settle/Refund`，经 [service/billing\_session.go](../../go/new-api/service/billing_session.go) 编排；月度不足 → 403 `ErrorCodeInsufficientUserQuota`（可触发钱包回退） |
| 套餐管理 API       | [controller/subscription.go](../../go/new-api/controller/subscription.go)                          | `AdminUpsertSubscriptionPlanRequest` 直接绑定 `model.SubscriptionPlan`，新增字段自动透传                                                                                                              |
| 前端（default 主题） | `web/default/src/features/subscriptions/`、`features/wallet/components/subscription-plans-card.tsx` | Zod schema + RHF 表单（mutate-drawer）+ 列表展示                                                                                                                                                 |
| 迁移             | [model/main.go](../../go/new-api/model/main.go)                                                    | `AutoMigrate` 已注册三张订阅表，新增列自动 ADD COLUMN                                                                                                                                                  |
| 测试基建           | [model/task\_cas\_test.go](../../go/new-api/model/task_cas_test.go)                                | `TestMain` 用 SQLite `:memory:` 且已 AutoMigrate 全部订阅表，可直接复用                                                                                                                                |
| classic 主题     | `web/classic/src/helpers/subscriptionFormat.js`                                                    | 仅被动展示时长/重置周期文案，无套餐 CRUD；新增字段为增量，classic 无需改动                                                                                                                                             |

## 三、方案设计

### 3.1 数据模型变更

**`SubscriptionPlan`** **新增 4 列**（配置）：

```go
// 三档限额模式开关
TieredLimitEnabled   bool  `json:"tiered_limit_enabled" gorm:"default:false"`
// 会话档点数上限（0 = 该档不限）
SessionLimitAmount   int64 `json:"session_limit_amount" gorm:"type:bigint;default:0"`
// 会话窗口时长，默认 18000 秒（5 小时）
SessionWindowSeconds int64 `json:"session_window_seconds" gorm:"type:bigint;default:18000"`
// 周档点数上限（0 = 该档不限）
WeeklyLimitAmount    int64 `json:"weekly_limit_amount" gorm:"type:bigint;default:0"`
```

**`UserSubscription`** **新增 9 列**（创建时快照套餐配置 + 三档运行计数）：

```go
// 套餐快照（创建时拷贝，与 AmountTotal 快照行为一致）
TieredLimitEnabled   bool  `json:"tiered_limit_enabled" gorm:"default:false"`
SessionLimitAmount   int64 `json:"session_limit_amount" gorm:"type:bigint;default:0"`
SessionWindowSeconds int64 `json:"session_window_seconds" gorm:"type:bigint;default:18000"`
WeeklyLimitAmount    int64 `json:"weekly_limit_amount" gorm:"type:bigint;default:0"`
// 会话档计数（SessionWindowStart=0 表示尚未开启会话）
SessionUsed        int64 `json:"session_used" gorm:"type:bigint;default:0"`
SessionWindowStart int64 `json:"session_window_start" gorm:"type:bigint;default:0"`
// 周档计数（WeekNextResetTime=0 表示未初始化，首次使用时设置）
WeeklyUsed        int64 `json:"weekly_used" gorm:"type:bigint;default:0"`
WeekNextResetTime int64 `json:"week_next_reset_time" gorm:"type:bigint;default:0"`
```

月档沿用现有 `AmountTotal/AmountUsed/NextResetTime`（三档模式下强制 `QuotaResetPeriod=monthly`，复用 `calcNextResetTime` 的自然月对齐与现有后台重置任务）。

**`SubscriptionPreConsumeRecord`** **新增 2 列**（退款锚点，跨窗口退款正确性的关键）：

```go
// 预扣发生时的窗口锚点；退款时窗口若已翻滚则不向该窗口回冲
SessionWindowStart int64 `json:"session_window_start" gorm:"type:bigint;default:0"`
WeekNextResetTime  int64 `json:"week_next_reset_time" gorm:"type:bigint;default:0"`
```

> 数值列一律 `default:0`、bool 列 `default:false`（避免 AGENTS.md 警告的 `default:true` 跨库布尔归一化问题；带默认值保证存量行回填非 NULL）。

### 3.2 窗口推进（惰性，均在订阅行 FOR UPDATE 事务内执行，天然并发安全）

新增两个纯函数（便于单测，写在该行上后一并 Save）：

* `advanceSessionWindow(sub *UserSubscription, nowUnix int64)`：

  * `SessionWindowStart == 0` → `SessionWindowStart = now, SessionUsed = 0`（首次请求锚定会话）

  * `now >= SessionWindowStart + SessionWindowSeconds` → `SessionWindowStart = now, SessionUsed = 0`（窗口到期，本次请求锚定新窗）

* `advanceWeeklyWindow(sub *UserSubscription, nowUnix int64)`：

  * `WeekNextResetTime == 0` → 初始化为下周一 00:00（复用 `calcNextResetTime` 的周一对齐算法）

  * `now >= WeekNextResetTime` → `WeeklyUsed = 0`，`WeekNextResetTime` 推进到下一个周一

* 月档：现有 `maybeResetUserSubscriptionWithPlanTx` 不变（三档套餐 period 已被归一化为 monthly）

> 惰性推进下，新会话窗锚定在"窗口过期后首次请求"时刻（而非过期瞬间），与方舟"以首次请求发生时间为周期起点"的表述一致，记为既定决策。

### 3.3 预扣费路径（`PreConsumeUserSubscription` 扩展）

单订阅候选通过条件由"月档余量 ≥ amount"变为（`sub.TieredLimitEnabled` 时）：

1. `maybeReset`（月档，现有）→ `advanceSessionWindow` → `advanceWeeklyWindow`
2. 三档校验（0 表示不限）：

   * 会话：`SessionLimitAmount == 0 || SessionLimitAmount - SessionUsed >= amount`

   * 周：`WeeklyLimitAmount == 0 || WeeklyLimitAmount - WeeklyUsed >= amount`

   * 月：`AmountTotal == 0 || AmountTotal - AmountUsed >= amount`（现有）
3. 通过 → record 写入两个锚点快照，三档计数同时 `+= amount`（同事务）
4. 任一档不足 → 返回**结构化错误**，不再继续尝试该订阅：

```go
// model 层定义，供 service 层 errors.Is 判断
type TierExhaustedError struct {
    Tier    string // "session" | "weekly"
    ResetAt int64  // 窗口恢复时间戳
}
func (e *TierExhaustedError) Error() string // 例如 "session tier exhausted, resets at 1710000000"
```

* 会话档不足 → `&TierExhaustedError{Tier: "session", ResetAt: SessionWindowStart + SessionWindowSeconds}`

* 周档不足 → `&TierExhaustedError{Tier: "weekly", ResetAt: WeekNextResetTime}`

* 月档不足 → 保持现有 `"subscription quota insufficient"` 错误文本（沿用现有 403/回退逻辑）

返回值 `SubscriptionPreConsumeResult` 增加字段：`SessionWindowStart`、`WeekNextResetTime`（供 `SubscriptionFunding` 结算/追加预扣时携带锚点）。

### 3.4 结算 / 追加预扣 / 退款

`PostConsumeUserSubscriptionDelta` 增加带锚点的变体（保留旧签名给非三档路径）：

```go
// PostConsumeUserSubscriptionTieredDelta：月档必调；会话/周档仅当订阅当前锚点与传入锚点一致才调整
// delta > 0 时三档均需校验余量，不足返回错误（含 TierExhaustedError）
func PostConsumeUserSubscriptionTieredDelta(subId int, delta int64, sessAnchor, weekAnchor int64) error
```

* **Settle**（`SubscriptionFunding.Settle`）：改调带锚点变体；窗口若已翻滚（锚点不匹配），会话/周档不动，仅月档调整——请求通常在分钟级完成，属边界保护

* **Reserve**（`BillingSession.reserveFunding` 的 Subscription 分支）：改调带锚点变体，正 delta 会在三档校验；流式请求中途会话档耗尽 → 返回 429 错误（见 3.5），流被中断，与钱包中途扣空的行为一致

* **Refund**（`RefundSubscriptionPreConsume`）：从 record 读出锚点 → 带锚点变体负 delta。窗口已翻滚的档不回冲（该窗口额度已重置，回冲会造成凭空增额）

### 3.5 错误映射与限流响应（billing\_session.go / funding\_source.go）

* `types` 包新增错误码：`ErrorCodeSubscriptionTierExhausted`

* `BillingSession.preConsume` / `reserveFunding` 中对 `*TierExhaustedError`（`errors.As`）返回：
  `types.NewErrorWithStatusCode(错误信息, ErrorCodeSubscriptionTierExhausted, http.StatusTooManyRequests, ErrOptionWithSkipRetry(), ErrOptionWithNoRecordErrorLog())`

  * 错误信息形如：`会话额度已用尽，将于 2024-01-01 12:00:00 恢复（约 2 小时后）`（周档同理）

  * `SkipRetry`：限流与渠道无关，重试无意义；`429` 让 Claude Code 等工具正确识别为限流

* **不回退钱包**：`NewBillingSession` 的钱包回退仅由 `ErrorCodeInsufficientUserQuota` 触发，三档错误码不同，天然不会触发回退；`AllowWalletOverflow` 对三档套餐仅在"无可用订阅"时生效（现有 `HasActiveUserSubscription` 路径），符合"直接限流拒绝"的决策

* 月档不足维持现有 403 `ErrorCodeInsufficientUserQuota` 行为不变

### 3.6 套餐创建/更新归一化与校验（controller/subscription.go）

`AdminCreateSubscriptionPlan` / `AdminUpdateSubscriptionPlan` 增加逻辑：

* `TieredLimitEnabled == true` 时：

  * 强制 `QuotaResetPeriod = SubscriptionResetMonthly`（三档套餐的月档语义，防止管理员误配）

  * 校验 `SessionWindowSeconds` 落在 `(0, 604800]`（默认 18000）

  * 校验 `SessionLimitAmount >= 0`、`WeeklyLimitAmount >= 0`、`SessionLimitAmount + WeeklyLimitAmount` 至少一个 > 0（否则三档无意义）

  * 校验 `SessionLimitAmount <= TotalAmount`、`WeeklyLimitAmount <= TotalAmount`（档位上限不应超过月度总额，TotalAmount>0 时）

### 3.7 用户/管理侧展示

* `CreateUserSubscriptionFromPlanTx`：拷贝 4 个套餐快照字段到 `UserSubscription`

* `buildSubscriptionSummaries`（`GetAllActiveUserSubscriptions` / `GetAllUserSubscriptions` 出口）做**只读归一化**：按当前时间虚拟推进窗口，输出计算字段（不写库）：

  ```go
  // SubscriptionSummary 内新增 TieredUsage *struct{...}（非三档为 nil）
  type SubscriptionTieredUsage struct {
      SessionLimit, SessionUsed, SessionResetTime  int64
      WeeklyLimit,  WeeklyUsed,  WeeklyResetTime   int64
      MonthlyLimit, MonthlyUsed, MonthlyResetTime  int64 // ResetTime=0 表示不限/未知
  }
  ```

  前端直接渲染"用量/上限 + 恢复时间"，避免在两端重复窗口推进逻辑

* 通知（`checkAndSendSubscriptionQuotaNotify`）、日志（`appendBillingInfo`）均基于月档字段，无需改动

### 3.8 前端（仅 web/default；classic 字段为增量 JSON，无需改动）

| 文件                                                                                                                     | 改动                                                                                                |
| ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `features/subscriptions/types.ts`                                                                                      | `subscriptionPlanSchema` 加 4 字段；`userSubscriptionSchema` 加快照/计数字段；新增 `SubscriptionTieredUsage` 类型 |
| `features/subscriptions/components/subscriptions-mutate-drawer.tsx`                                                    | 新增「三档限额」开关；开启后显示：会话点数上限、会话窗口时长（小时，默认 5）、周点数上限，并锁定重置周期为"每月"（禁用选择器）                                 |
| `features/subscriptions/lib/plan-form.ts`                                                                              | 默认值与校验（窗口 1\~168 小时、至少一档上限 > 0）                                                                   |
| `features/subscriptions/components/subscriptions-columns.tsx`、`features/wallet/components/subscription-plans-card.tsx` | 三档套餐渲染三条进度（会话/周/月：used/limit + 恢复时间倒计时格式化），非三档保持现有展示                                              |
| `web/default/src/i18n/locales/{en,zh,fr,ru,ja,vi}.json`                                                                | 全部新增文案（flat JSON，英文原文作 key），用 `bun run i18n:sync` 补齐                                              |

新增 UI 文案清单（英文 key）：`Tiered Usage Limits`、`Session Limit (points)`、`Session Window (hours)`、`Weekly Limit (points)`、`Monthly Limit (points)`、`Session usage`、`Weekly usage`、`Monthly usage`、`Resets at {time}`、`Session tier exhausted, resets at {time}`、`Weekly tier exhausted, resets at {time}` 等（以后端实际报错文案为准同步）。

## 四、实施步骤（按序执行）

1. **model/subscription.go**：新增三表列定义；`TierExhaustedError`；`advanceSessionWindow`/`advanceWeeklyWindow` 纯函数；`CreateUserSubscriptionFromPlanTx` 快照拷贝；`PreConsumeUserSubscription` 三档校验/计数/锚点写入/结构化错误；`PostConsumeUserSubscriptionTieredDelta`；`RefundSubscriptionPreConsume` 带锚点退款；`buildSubscriptionSummaries` 只读归一化输出 `TieredUsage`
2. **types（错误码）**：新增 `ErrorCodeSubscriptionTierExhausted`
3. **service/funding\_source.go**：`SubscriptionFunding` 保存锚点（来自 `SubscriptionPreConsumeResult` 新字段），`Settle` 改调带锚点变体
4. **service/billing\_session.go**：`preConsume` 与 `reserveFunding` 对 `TierExhaustedError` 的 429 映射（`errors.As`）；`reserveFunding` Subscription 分支携带锚点
5. **controller/subscription.go**：套餐创建/更新的三档归一化与校验（3.6）
6. **前端**：按 3.8 表格逐文件实施；i18n 6 语言补齐
7. **测试**（见第五节）

## 五、测试（遵循 AGENTS.md：testify require/assert、确定性表驱动）

在 `model` 包新增 `subscription_tiered_test.go`（复用 `task_cas_test.go` 的 `TestMain` fixture）：

* `TestAdvanceSessionWindow` / `TestAdvanceWeeklyWindow`：纯函数边界（未初始化锚定、到期重置、周一对齐）

* `TestTieredPreConsumeAndExhaust`：表驱动——正常三档同扣；会话档耗尽（校验 `TierExhaustedError.Tier=="session"` 与 ResetAt）；周档耗尽；月档耗尽（现有文本错误）；幂等重放

* `TestTieredRefundAcrossWindowRoll`：预扣后人工把 `SessionWindowStart` 翻窗，退款仅回冲月档；周档同理

* `TestTieredReserveDelta`：正 delta 三档校验不足时报错

service 层（若现有测试文件可挂载）：`BillingSession` 对 `TierExhaustedError` → 429 + SkipRetry 的映射断言（可用轻量单测直接构造错误走映射分支）。

## 六、假设与决策（已定，执行时无需再问）

1. 计量单位 = quota 点数（按 token 用量计费）；不实现"按请求次数"计费
2. 会话窗 = 锚定窗（首次请求起 N 小时，默认 5h，套餐可配 1\~168h）；到期后新窗锚定在下次请求
3. 周档对齐服务器本地时区周一 00:00；月档对齐自然月 1 日 00:00（与现有 monthly 重置一致，不引入"订阅月"概念）
4. 任一档耗尽 → 429 + 恢复时间提示，绝不回退钱包（新错误码与 `ErrorCodeInsufficientUserQuota` 区分实现）
5. 三档套餐强制 `quota_reset_period = monthly`；月度总额 = `TotalAmount`
6. 套餐快照模式：限额配置在购买时快照到 `UserSubscription`（与现有 `AmountTotal`/`AllowWalletOverflow` 快照行为一致），改套餐不影响存量订阅
7. 退款/结算遇窗口翻滚：翻滚档不回冲，月档必调
8. 0 值上限 = 该档不限（月档沿用现有 `AmountTotal == 0 = 不限` 语义）
9. classic 主题零改动（增量字段）；不做模型抵扣系数（Ark 的按模型差异化倍率）——留作后续扩展
10. 不改通知与日志模块（均基于月档，行为兼容）

## 七、验证

```bash
# 后端
go build ./...
go vet ./model/ ./service/ ./controller/
go test ./model/ -run 'Tiered|AdvanceSession|AdvanceWeekly' -v
go test ./service/ -run 'Tiered' -v   # 若实现 service 层映射测试

# 前端（web/default）
bun run i18n:sync && bun run typecheck && bun run lint && bun run build
```

手动验收（管理员后台）：

1. 创建三档套餐（如：总额 100 万点、会话 10 万/5h、周 40 万）→ 校验重置周期被锁定为"每月"
2. 绑定用户 → 用户侧订阅卡显示三条档位进度与恢复时间
3. 用小额套餐（会话上限设 1 点）触发调用 → 客户端收到 429，错误体含「会话额度已用尽，将于 X 恢复」
4. 把 `SessionWindowSeconds` 调成 60 秒的测试套餐验证窗口到期自动恢复；周一 00:00 验证周档重置
5. 回归：创建普通（非三档）套餐 → 预扣/结算/退款/重置行为与改动前一致；多订阅遍历、钱包回退路径不回归

