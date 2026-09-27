package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

const (
	BillingTypePrepaid  = "prepaid"
	BillingTypePostpaid = "postpaid"
)

const (
	// 对账单状态: 生成后待确认, 管理员确认后为已确认; voided 为作废终态
	StatementStatusPending   = "pending"
	StatementStatusConfirmed = "confirmed"
	StatementStatusVoided    = "voided"
)

// BillingStatement 客户对账单(预付费/后付费均可出账),出账时固化聚合快照,后续不依赖 logs 表
type BillingStatement struct {
	Id               int    `json:"id"`
	StatementNo      string `json:"statement_no" gorm:"unique;type:varchar(64);index"` // ST-{YYYYMM}-{userId}-{8位随机}
	UserId           int    `json:"user_id" gorm:"index"`
	Username         string `json:"username" gorm:"index;type:varchar(64)"`
	PeriodStart      int64  `json:"period_start"` // 秒级时间戳,含
	PeriodEnd        int64  `json:"period_end"`   // 秒级时间戳,不含(便于连续账期)
	ConsumeQuota     int64  `json:"consume_quota"`
	RefundQuota      int64  `json:"refund_quota"`
	TopupQuota       int64  `json:"topup_quota"`  // 周期内充值总额,仅参考展示,不计入应收
	AdjustQuota      int64  `json:"adjust_quota"` // 调整行合计,可负
	TotalQuota       int64  `json:"total_quota"`  // 应收 = Consume - Refund + Adjust
	RequestCount     int64  `json:"request_count"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
	ModelBreakdown   string `json:"model_breakdown" gorm:"type:text"` // 按模型分组 JSON 快照
	Status           string `json:"status" gorm:"type:varchar(16);index;default:'pending'"`
	SettledQuota     int64  `json:"settled_quota"` // 标记已确认且勾选结清时 = TotalQuota,否则 0
	PaidAt           int64  `json:"paid_at"`
	PaidNote         string `json:"paid_note" gorm:"type:varchar(255)"`
	Remark           string `json:"remark" gorm:"type:varchar(255)"`
	CreatedAt        int64  `json:"created_at" gorm:"index"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (stmt *BillingStatement) TableName() string {
	return "billing_statements"
}

// StatementAdjustment 账单调整行(折扣/减免为负,加价/罚金为正)
type StatementAdjustment struct {
	Id          int    `json:"id"`
	StatementId int    `json:"statement_id" gorm:"index"`
	Amount      int64  `json:"amount"` // quota
	Reason      string `json:"reason" gorm:"type:varchar(255)"`
	CreatedAt   int64  `json:"created_at"`
	CreatedBy   string `json:"created_by" gorm:"type:varchar(64)"`
}

func (adj *StatementAdjustment) TableName() string {
	return "statement_adjustments"
}

// StatementModelRow 按模型分组的聚合行,出账时固化进 ModelBreakdown
type StatementModelRow struct {
	Model            string `json:"model"`
	Quota            int64  `json:"quota"`
	Requests         int64  `json:"requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

// StatementAggregation 账期内用户用量的完整聚合结果
type StatementAggregation struct {
	ConsumeQuota     int64               `json:"consume_quota"`
	RefundQuota      int64               `json:"refund_quota"`
	TopupQuota       int64               `json:"topup_quota"`
	RequestCount     int64               `json:"request_count"`
	PromptTokens     int64               `json:"prompt_tokens"`
	CompletionTokens int64               `json:"completion_tokens"`
	ModelRows        []StatementModelRow `json:"model_rows"`
}

// AggregateUserUsage 聚合 [start, end) 账期内用户的消费/退款/按模型分组明细与充值参考
func AggregateUserUsage(userId int, start, end int64) (*StatementAggregation, error) {
	agg := &StatementAggregation{}

	// 空切片初始化,保证无记录时 JSON 序列化为 [] 而非 null(前端表格对 null 取 length 会崩溃)
	modelRows := make([]StatementModelRow, 0)
	err := LOG_DB.Table("logs").
		Select("model_name as model, COALESCE(SUM(quota), 0) as quota, COUNT(*) as requests, COALESCE(SUM(prompt_tokens), 0) as prompt_tokens, COALESCE(SUM(completion_tokens), 0) as completion_tokens").
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?", userId, LogTypeConsume, start, end).
		Group("model_name").Order("quota desc").Scan(&modelRows).Error
	if err != nil {
		return nil, err
	}
	agg.ModelRows = modelRows
	for _, row := range modelRows {
		agg.ConsumeQuota += row.Quota
		agg.RequestCount += row.Requests
		agg.PromptTokens += row.PromptTokens
		agg.CompletionTokens += row.CompletionTokens
	}

	err = LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota), 0)").
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at < ?", userId, LogTypeRefund, start, end).
		Scan(&agg.RefundQuota).Error
	if err != nil {
		return nil, err
	}

	err = DB.Model(&TopUp{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND status = ? AND complete_time >= ? AND complete_time < ?", userId, common.TopUpStatusSuccess, start, end).
		Scan(&agg.TopupQuota).Error
	if err != nil {
		return nil, err
	}

	return agg, nil
}

// CreateBillingStatement 出账:聚合一次并固化快照落库。同用户非作废账单账期重叠时拒绝。
func CreateBillingStatement(userId int, start, end int64, remark string) (*BillingStatement, error) {
	if start >= end {
		return nil, errors.New("账期开始时间必须早于结束时间")
	}
	user, err := GetUserById(userId, false)
	if err != nil {
		return nil, errors.New("用户不存在")
	}

	var overlap BillingStatement
	err = DB.Where("user_id = ? AND status <> ? AND period_start < ? AND period_end > ?",
		userId, StatementStatusVoided, end, start).First(&overlap).Error
	if err == nil {
		return nil, fmt.Errorf("账期与已有账单 %s 重叠", overlap.StatementNo)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	agg, err := AggregateUserUsage(userId, start, end)
	if err != nil {
		return nil, err
	}
	modelBreakdown, err := common.Marshal(agg.ModelRows)
	if err != nil {
		return nil, err
	}

	now := common.GetTimestamp()
	stmt := &BillingStatement{
		StatementNo:      fmt.Sprintf("ST-%s-%d-%s", time.Unix(start, 0).Format("200601"), userId, common.GetRandomString(8)),
		UserId:           userId,
		Username:         user.Username,
		PeriodStart:      start,
		PeriodEnd:        end,
		ConsumeQuota:     agg.ConsumeQuota,
		RefundQuota:      agg.RefundQuota,
		TopupQuota:       agg.TopupQuota,
		TotalQuota:       agg.ConsumeQuota - agg.RefundQuota,
		RequestCount:     agg.RequestCount,
		PromptTokens:     agg.PromptTokens,
		CompletionTokens: agg.CompletionTokens,
		ModelBreakdown:   string(modelBreakdown),
		Status:           StatementStatusPending,
		Remark:           remark,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := DB.Create(stmt).Error; err != nil {
		return nil, err
	}
	return stmt, nil
}

func GetBillingStatementsPage(pageInfo *common.PageInfo, userId int, status string, start, end int64) ([]*BillingStatement, int64, error) {
	tx := DB.Model(&BillingStatement{})
	if userId != 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if start != 0 {
		tx = tx.Where("period_end > ?", start)
	}
	if end != 0 {
		tx = tx.Where("period_start < ?", end)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var statements []*BillingStatement
	err := tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&statements).Error
	return statements, total, err
}

func GetBillingStatementById(id int) (*BillingStatement, []*StatementAdjustment, error) {
	stmt := &BillingStatement{}
	if err := DB.First(stmt, "id = ?", id).Error; err != nil {
		return nil, nil, err
	}
	// 空切片初始化,避免无调整行时 JSON 序列化为 null
	adjustments := make([]*StatementAdjustment, 0)
	if err := DB.Where("statement_id = ?", id).Order("id asc").Find(&adjustments).Error; err != nil {
		return nil, nil, err
	}
	return stmt, adjustments, nil
}

// recomputeStatementTotals 根据调整行重算 AdjustQuota 与 TotalQuota
func recomputeStatementTotals(tx *gorm.DB, stmt *BillingStatement) error {
	var adjustSum int64
	err := tx.Model(&StatementAdjustment{}).Where("statement_id = ?", stmt.Id).
		Select("COALESCE(SUM(amount), 0)").Scan(&adjustSum).Error
	if err != nil {
		return err
	}
	stmt.AdjustQuota = adjustSum
	stmt.TotalQuota = stmt.ConsumeQuota - stmt.RefundQuota + adjustSum
	return tx.Model(&BillingStatement{}).Where("id = ?", stmt.Id).Updates(map[string]interface{}{
		"adjust_quota": adjustSum,
		"total_quota":  stmt.TotalQuota,
		"updated_at":   common.GetTimestamp(),
	}).Error
}

func AddStatementAdjustment(stmtId int, amount int64, reason string, createdBy string) (*BillingStatement, error) {
	stmt := &BillingStatement{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(stmt, "id = ?", stmtId).Error; err != nil {
			return err
		}
		if stmt.Status != StatementStatusPending {
			return errors.New("只有待确认账单才能添加调整")
		}
		adjustment := &StatementAdjustment{
			StatementId: stmtId,
			Amount:      amount,
			Reason:      reason,
			CreatedAt:   common.GetTimestamp(),
			CreatedBy:   createdBy,
		}
		if err := tx.Create(adjustment).Error; err != nil {
			return err
		}
		return recomputeStatementTotals(tx, stmt)
	})
	if err != nil {
		return nil, err
	}
	return stmt, nil
}

func DeleteStatementAdjustment(stmtId int, adjId int) (*BillingStatement, error) {
	stmt := &BillingStatement{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(stmt, "id = ?", stmtId).Error; err != nil {
			return err
		}
		if stmt.Status != StatementStatusPending {
			return errors.New("只有待确认账单才能删除调整")
		}
		result := tx.Where("id = ? AND statement_id = ?", adjId, stmtId).Delete(&StatementAdjustment{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.New("调整行不存在")
		}
		return recomputeStatementTotals(tx, stmt)
	})
	if err != nil {
		return nil, err
	}
	return stmt, nil
}

// ConfirmStatement 确认账单;settle 为 true 时在事务内把应收欠款补回用户余额
func ConfirmStatement(stmtId int, settle bool, note string) (*BillingStatement, error) {
	stmt := &BillingStatement{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(stmt, "id = ?", stmtId).Error; err != nil {
			return err
		}
		if stmt.Status != StatementStatusPending {
			return errors.New("只有待确认账单才能确认")
		}
		now := common.GetTimestamp()
		updates := map[string]interface{}{
			"status":        StatementStatusConfirmed,
			"paid_at":       now,
			"paid_note":     note,
			"updated_at":    now,
			"settled_quota": 0,
		}
		if settle {
			updates["settled_quota"] = stmt.TotalQuota
			if err := tx.Model(&User{}).Where("id = ?", stmt.UserId).
				Update("quota", gorm.Expr("quota + ?", stmt.TotalQuota)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&BillingStatement{}).Where("id = ?", stmtId).Updates(updates).Error; err != nil {
			return err
		}
		stmt.Status = StatementStatusConfirmed
		stmt.PaidAt = now
		stmt.PaidNote = note
		stmt.SettledQuota = 0
		if settle {
			stmt.SettledQuota = stmt.TotalQuota
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if settle && stmt.TotalQuota != 0 {
		if err := cacheIncrUserQuota(stmt.UserId, stmt.TotalQuota); err != nil {
			common.SysLog(fmt.Sprintf("failed to sync user quota cache after statement %s confirmed: %s", stmt.StatementNo, err.Error()))
		}
		RecordLog(stmt.UserId, LogTypeManage, fmt.Sprintf("账单 %s 已确认, 结转欠费 %s", stmt.StatementNo, logger.FormatQuota(int(stmt.TotalQuota))))
	} else {
		RecordLog(stmt.UserId, LogTypeManage, fmt.Sprintf("账单 %s 已确认", stmt.StatementNo))
	}
	return stmt, nil
}

func VoidStatement(stmtId int) (*BillingStatement, error) {
	stmt := &BillingStatement{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(stmt, "id = ?", stmtId).Error; err != nil {
			return err
		}
		if stmt.Status != StatementStatusPending {
			return errors.New("只有待确认账单才能作废")
		}
		if err := tx.Model(&BillingStatement{}).Where("id = ?", stmtId).Updates(map[string]interface{}{
			"status":     StatementStatusVoided,
			"updated_at": common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		stmt.Status = StatementStatusVoided
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stmt, nil
}

// StatementUserSummary 生成对账单的用户下拉数据:预付费/后付费均可出账,附待确认账单统计
type StatementUserSummary struct {
	Id            int    `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	BillingType   string `json:"billing_type"`
	Quota         int    `json:"quota"`
	CreditLimit   int64  `json:"credit_limit"`
	PendingCount  int64  `json:"pending_count"`
	PendingTotal  int64  `json:"pending_total"`
}

func GetStatementUsers() ([]StatementUserSummary, error) {
	// 空切片初始化,避免无用户时 JSON 序列化为 null
	users := make([]StatementUserSummary, 0)
	err := DB.Model(&User{}).
		Select("id, username, display_name, billing_type, quota, credit_limit").
		Order("id asc").Scan(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return users, nil
	}

	var stats []struct {
		UserId int   `gorm:"column:user_id"`
		Cnt    int64 `gorm:"column:cnt"`
		Sum    int64 `gorm:"column:sum"`
	}
	err = DB.Model(&BillingStatement{}).
		Select("user_id, COUNT(*) as cnt, COALESCE(SUM(total_quota), 0) as sum").
		Where("status = ?", StatementStatusPending).Group("user_id").Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	pendingByUser := make(map[int]struct {
		Cnt int64
		Sum int64
	}, len(stats))
	for _, s := range stats {
		pendingByUser[s.UserId] = struct {
			Cnt int64
			Sum int64
		}{Cnt: s.Cnt, Sum: s.Sum}
	}
	for i := range users {
		if s, ok := pendingByUser[users[i].Id]; ok {
			users[i].PendingCount = s.Cnt
			users[i].PendingTotal = s.Sum
		}
	}
	return users, nil
}
