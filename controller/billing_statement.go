package controller

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// GetPostpaidBillingUsers 返回后付费客户列表(生成账单对话框的数据源)
func GetPostpaidBillingUsers(c *gin.Context) {
	users, err := model.GetPostpaidUsers()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, users)
}

// PreviewBillingStatement 预览账期聚合结果(不落库),附带用户当前余额与信用额度
func PreviewBillingStatement(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Query("user_id"))
	start, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	if userId <= 0 || start <= 0 || end <= start {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	agg, err := model.AggregateUserUsage(userId, start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"user_id":                userId,
		"username":               user.Username,
		"quota":                  user.Quota,
		"credit_limit":           user.CreditLimit,
		"period_start":           start,
		"period_end":             end,
		"total_quota":            agg.ConsumeQuota - agg.RefundQuota,
		"aggregation":            agg,
	})
}

type CreateBillingStatementRequest struct {
	UserId      int    `json:"user_id"`
	PeriodStart int64  `json:"period_start"`
	PeriodEnd   int64  `json:"period_end"`
	Remark      string `json:"remark"`
}

// CreateBillingStatement 出账:聚合账期用量并固化快照落库
func CreateBillingStatement(c *gin.Context) {
	var req CreateBillingStatementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.UserId <= 0 || req.PeriodStart <= 0 || req.PeriodEnd <= req.PeriodStart {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	stmt, err := model.CreateBillingStatement(req.UserId, req.PeriodStart, req.PeriodEnd, strings.TrimSpace(req.Remark))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stmt)
}

// GetAllBillingStatements 账单分页列表
func GetAllBillingStatements(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId, _ := strconv.Atoi(c.Query("user_id"))
	status := c.Query("status")
	start, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	end, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	statements, total, err := model.GetBillingStatementsPage(pageInfo, userId, status, start, end)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(statements)
	common.ApiSuccess(c, pageInfo)
}

// GetBillingStatementDetail 账单详情(含调整行)
func GetBillingStatementDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	stmt, adjustments, err := model.GetBillingStatementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"statement":   stmt,
		"adjustments": adjustments,
	})
}

type UpdateStatementStatusRequest struct {
	Status string `json:"status"` // paid / voided
	Settle bool   `json:"settle"`
	Note   string `json:"note"`
}

// UpdateBillingStatementStatus 结清(paid)或作废(voided),均为单向终态
func UpdateBillingStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var req UpdateStatementStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var stmt *model.BillingStatement
	switch req.Status {
	case model.StatementStatusPaid:
		stmt, err = model.MarkStatementPaid(id, req.Settle, strings.TrimSpace(req.Note))
	case model.StatementStatusVoided:
		stmt, err = model.VoidStatement(id)
	default:
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stmt)
}

type AddStatementAdjustmentRequest struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
}

// AddBillingStatementAdjustment 添加调整行(负=减免/折扣,正=加价/罚金),实时重算应收
func AddBillingStatementAdjustment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	var req AddStatementAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	if req.Amount == 0 || strings.TrimSpace(req.Reason) == "" {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	stmt, err := model.AddStatementAdjustment(id, req.Amount, strings.TrimSpace(req.Reason), c.GetString("username"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stmt)
}

// DeleteBillingStatementAdjustment 删除调整行并重算应收
func DeleteBillingStatementAdjustment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	adjId, adjErr := strconv.Atoi(c.Param("adj_id"))
	if err != nil || adjErr != nil || id <= 0 || adjId <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	stmt, err := model.DeleteStatementAdjustment(id, adjId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stmt)
}

// ExportBillingStatementCsv 导出账单 CSV(UTF-8 BOM,金额为人民币两位小数)
func ExportBillingStatementCsv(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	stmt, adjustments, err := model.GetBillingStatementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var modelRows []model.StatementModelRow
	if stmt.ModelBreakdown != "" {
		if err := common.UnmarshalJsonStr(stmt.ModelBreakdown, &modelRows); err != nil {
			common.ApiError(c, fmt.Errorf("账单明细解析失败: %s", err.Error()))
			return
		}
	}

	statusText := map[string]string{
		model.StatementStatusUnpaid: "未结清",
		model.StatementStatusPaid:   "已结清",
		model.StatementStatusVoided: "已作废",
	}
	formatMoney := func(quota int64) string {
		return fmt.Sprintf("%.2f", float64(quota)/common.QuotaPerUnit*operation_setting.USDExchangeRate)
	}
	formatTime := func(ts int64) string {
		return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM, 保证 Excel 打开中文不乱码
	w := csv.NewWriter(&buf)

	quotaCell := func(label string, quota int64) []string {
		return []string{label + " (CNY)", formatMoney(quota)}
	}
	rows := [][]string{
		{"账单号", stmt.StatementNo},
		{"用户", fmt.Sprintf("%s (ID: %d)", stmt.Username, stmt.UserId)},
		{"账期", formatTime(stmt.PeriodStart) + " ~ " + formatTime(stmt.PeriodEnd)},
		{"状态", statusText[stmt.Status]},
		quotaCell("消费", stmt.ConsumeQuota),
		quotaCell("退款", stmt.RefundQuota),
		quotaCell("调整", stmt.AdjustQuota),
		quotaCell("应收", stmt.TotalQuota),
		{"请求数", strconv.FormatInt(stmt.RequestCount, 10)},
		{"输入 tokens", strconv.FormatInt(stmt.PromptTokens, 10)},
		{"输出 tokens", strconv.FormatInt(stmt.CompletionTokens, 10)},
		quotaCell("周期内充值参考", stmt.TopupQuota),
	}
	if stmt.Remark != "" {
		rows = append(rows, []string{"备注", stmt.Remark})
	}
	if stmt.Status == model.StatementStatusPaid {
		rows = append(rows,
			[]string{"收款时间", formatTime(stmt.PaidAt)},
			[]string{"收款备注", stmt.PaidNote},
			quotaCell("结转欠费", stmt.SettledQuota),
		)
	}
	rows = append(rows, []string{})
	if err := w.WriteAll(rows); err != nil {
		common.ApiError(c, err)
		return
	}

	if len(modelRows) > 0 {
		if err := w.Write([]string{}); err != nil {
			common.ApiError(c, err)
			return
		}
		if err := w.Write([]string{"模型", "金额 (CNY)", "请求数", "输入 tokens", "输出 tokens"}); err != nil {
			common.ApiError(c, err)
			return
		}
		for _, row := range modelRows {
			if err := w.Write([]string{
				row.Model,
				formatMoney(row.Quota),
				strconv.FormatInt(row.Requests, 10),
				strconv.FormatInt(row.PromptTokens, 10),
				strconv.FormatInt(row.CompletionTokens, 10),
			}); err != nil {
				common.ApiError(c, err)
				return
			}
		}
	}

	if len(adjustments) > 0 {
		if err := w.Write([]string{}); err != nil {
			common.ApiError(c, err)
			return
		}
		if err := w.Write([]string{"调整原因", "金额 (CNY)", "操作人", "时间"}); err != nil {
			common.ApiError(c, err)
			return
		}
		for _, adj := range adjustments {
			if err := w.Write([]string{
				adj.Reason,
				formatMoney(adj.Amount),
				adj.CreatedBy,
				formatTime(adj.CreatedAt),
			}); err != nil {
				common.ApiError(c, err)
				return
			}
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		common.ApiError(c, err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=statement-%s.csv", stmt.StatementNo))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}
