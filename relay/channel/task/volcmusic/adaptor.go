package volcmusic

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	defaultHost = "https://open.volcengineapi.com"
	apiVersion  = "2024-08-12"
	actionSong  = "GenSongForTime"
	actionBGM   = "GenBGMForTime"
	actionQuery = "QuerySong"

	musicTypeSong = "song"
	musicTypeBGM  = "bgm"
)

// TaskAdaptor 实现火山引擎音乐生成的任务型适配器。
// 该 API 为异步任务型，使用火山引擎 HMAC-SHA256 签名鉴权（region=cn-beijing, service=imagination）。
// 计费方式：预扣为正常额度的 1/100，任务完成后按实际 Duration（秒数）× ModelRatio × GroupRatio 差额结算。
type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	accessKey   string
	secretKey   string
	baseURL     string
	// musicType 由 ValidateRequestAndSetAction 解析，决定调用 GenSongForTime 还是 GenBGMForTime。
	musicType string
}

// preChargeRatio 预扣比例：预扣仅为正常额度的 1/100，避免长时间任务占用过多额度。
const preChargeRatio = 0.01

// EstimateBilling 设置预扣比例为 1/100。
func (a *TaskAdaptor) EstimateBilling(_ *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	return map[string]float64{
		"pre_charge_ratio": preChargeRatio,
	}
}

// AdjustBillingOnComplete 任务完成后按实际 Duration（秒数）× ModelRatio × GroupRatio 计算实际费用。
// 不使用预扣时的 0.01 系数，确保最终计费准确。
// 返回正值后框架会通过 RecalculateTaskQuota 做差额结算（多退少补）。
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if taskResult.TotalTokens <= 0 {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if bc == nil {
		return 0
	}
	// 实际费用 = Duration(秒) × ModelRatio × GroupRatio（不乘预扣比例）
	actualQuota := int(float64(taskResult.TotalTokens) * bc.ModelRatio * bc.GroupRatio)
	return actualQuota
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.musicType = musicTypeSong

	// apiKey 格式: "accessKey|secretKey"
	keyParts := strings.Split(info.ApiKey, "|")
	if len(keyParts) == 2 {
		a.accessKey = strings.TrimSpace(keyParts[0])
		a.secretKey = strings.TrimSpace(keyParts[1])
	}
}

// ValidateRequestAndSetAction 解析标准 TaskSubmitReq，校验字段并确定生成类型。
// metadata.type 决定调用 GenSongForTime(song) 或 GenBGMForTime(bgm)，默认 song。
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	if strings.TrimSpace(req.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field model is required"), "invalid_request", http.StatusBadRequest)
	}

	musicType := musicTypeSong
	if req.Metadata != nil {
		if t, ok := req.Metadata["type"].(string); ok {
			t = strings.ToLower(strings.TrimSpace(t))
			if t != "" {
				musicType = t
			}
		}
	}
	if musicType != musicTypeSong && musicType != musicTypeBGM {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("invalid metadata.type: %s, must be song or bgm", musicType),
			"invalid_request", http.StatusBadRequest,
		)
	}

	// bgm 模式下 Text(对应 Prompt)为火山引擎必选字段。
	// song 模式下 Prompt 与 Lyrics 均可选，但至少需提供其一。
	if musicType == musicTypeBGM {
		if strings.TrimSpace(req.Prompt) == "" {
			return service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required for bgm type"), "invalid_request", http.StatusBadRequest)
		}
	} else {
		hasLyrics := false
		if req.Metadata != nil {
			if l, ok := req.Metadata["lyrics"].(string); ok && strings.TrimSpace(l) != "" {
				hasLyrics = true
			}
		}
		if strings.TrimSpace(req.Prompt) == "" && !hasLyrics {
			return service.TaskErrorWrapperLocal(fmt.Errorf("prompt or lyrics is required for song type"), "invalid_request", http.StatusBadRequest)
		}
	}

	a.musicType = musicType
	info.Action = constant.TaskActionGenerate
	info.OriginModelName = req.Model
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	action := actionSong
	if a.musicType == musicTypeBGM {
		action = actionBGM
	}
	return buildActionURL(a.baseURL, action), nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if a.accessKey == "" || a.secretKey == "" {
		return fmt.Errorf("invalid api key format for volcmusic: expected 'ak|sk'")
	}
	return SignRequest(req, a.accessKey, a.secretKey)
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, _ *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	var data []byte
	if a.musicType == musicTypeBGM {
		data, err = common.Marshal(buildBGMRequest(&req))
	} else {
		data, err = common.Marshal(buildSongRequest(&req))
	}
	if err != nil {
		return nil, errors.Wrap(err, "marshal request body failed")
	}
	common.SysLog(fmt.Sprintf("[volcmusic] upstream request body: %s", string(data)))
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse 处理提交响应，返回上游 TaskID 并向客户端返回标准 task 响应。
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	common.SysLog(fmt.Sprintf("[volcmusic] submit response body: %s", strings.TrimSpace(string(responseBody))))

	if resp.StatusCode != http.StatusOK {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody))),
			"upstream_error", resp.StatusCode,
		)
		return
	}

	var sResp SubmitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if sResp.Code != 0 {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("%s", sResp.Message), fmt.Sprintf("%d", sResp.Code), http.StatusInternalServerError)
		return
	}

	taskID = sResp.Result.TaskID
	if taskID == "" {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("task_id is empty in upstream response, body: %s", strings.TrimSpace(string(responseBody))),
			"invalid_response", http.StatusInternalServerError,
		)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)

	return taskID, responseBody, nil
}

// FetchTask 查询任务状态。火山引擎查询同样需要签名。
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := buildActionURL(baseUrl, actionQuery)
	payloadBytes, err := common.Marshal(QueryTaskRequest{TaskID: taskID})
	if err != nil {
		return nil, errors.Wrap(err, "marshal query task payload failed")
	}

	req, err := http.NewRequest(http.MethodPost, uri, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	keyParts := strings.Split(key, "|")
	if len(keyParts) != 2 {
		return nil, fmt.Errorf("invalid api key format for volcmusic: expected 'ak|sk'")
	}
	accessKey := strings.TrimSpace(keyParts[0])
	secretKey := strings.TrimSpace(keyParts[1])

	if err := SignRequest(req, accessKey, secretKey); err != nil {
		return nil, errors.Wrap(err, "sign request failed")
	}

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

// ParseTaskResult 解析查询响应，将火山引擎 Status 映射到内部任务状态，
// 并将 SongDetail.Duration（秒数）转为 int 作为 TotalTokens / CompletionTokens 用于计费。
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	common.SysLog(fmt.Sprintf("[volcmusic] fetch response body: %s", strings.TrimSpace(string(respBody))))

	var qResp QueryResponse
	if err := common.Unmarshal(respBody, &qResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	result := relaycommon.TaskInfo{}

	if qResp.Code != 0 {
		result.Code = qResp.Code
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = qResp.Message
		return &result, nil
	}

	result.Code = 0
	switch qResp.Result.Status {
	case 0: // 等待中
		result.Status = model.TaskStatusQueued
		result.Progress = "10%"
	case 1: // 处理中
		result.Status = model.TaskStatusInProgress
		result.Progress = "50%"
	case 2: // 成功
		result.Status = model.TaskStatusSuccess
		result.Progress = "100%"
		if qResp.Result.SongDetail != nil {
			result.Url = qResp.Result.SongDetail.AudioUrl
			tokens := int(qResp.Result.SongDetail.Duration)
			result.TotalTokens = tokens
			result.CompletionTokens = tokens
		}
	case 3: // 失败
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		if qResp.Result.FailureReason != nil {
			result.Reason = qResp.Result.FailureReason.Msg
		} else {
			result.Reason = qResp.Message
		}
	default:
		result.Status = model.TaskStatusInProgress
		result.Progress = "30%"
	}

	return &result, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// buildActionURL 构造包含 Action / Version 查询参数的火山引擎请求 URL。
func buildActionURL(base, action string) string {
	if base == "" {
		base = defaultHost
	}
	base = strings.TrimRight(base, "/")
	return fmt.Sprintf("%s/?Action=%s&Version=%s", base, action, apiVersion)
}

// buildSongRequest 将标准 TaskSubmitReq 转换为 GenSongForTime 的 PascalCase 请求体。
func buildSongRequest(req *relaycommon.TaskSubmitReq) *GenSongRequest {
	body := &GenSongRequest{
		Prompt: req.Prompt,
	}
	if d := req.Duration; d > 0 {
		body.Duration = &d
	}
	if req.Metadata == nil {
		return body
	}

	body.Lyrics = stringFromMeta(req.Metadata, "lyrics")
	body.ModelVersion = stringFromMeta(req.Metadata, "model_version")
	body.Genre = stringFromMeta(req.Metadata, "genre")
	body.Mood = stringFromMeta(req.Metadata, "mood")
	body.Gender = stringFromMeta(req.Metadata, "gender")
	body.Timbre = stringFromMeta(req.Metadata, "timbre")
	body.TosBucket = stringFromMeta(req.Metadata, "tos_bucket")
	body.CallbackURL = stringFromMeta(req.Metadata, "callback_url")
	body.Key = stringFromMeta(req.Metadata, "key")
	body.Scene = stringFromMeta(req.Metadata, "scene")
	body.Lang = stringFromMeta(req.Metadata, "lang")

	if v, ok := intFromMeta(req.Metadata, "kmode"); ok {
		body.Kmode = &v
	}
	if v, ok := intFromMeta(req.Metadata, "tempo"); ok {
		body.Tempo = &v
	}
	if v, ok := boolFromMeta(req.Metadata, "instrument"); ok {
		body.Instrument = &v
	}
	// metadata.duration 作为 duration 的回退来源
	if body.Duration == nil {
		if v, ok := intFromMeta(req.Metadata, "duration"); ok {
			body.Duration = &v
		}
	}
	return body
}

// buildBGMRequest 将标准 TaskSubmitReq 转换为 GenBGMForTime 的 PascalCase 请求体。
func buildBGMRequest(req *relaycommon.TaskSubmitReq) *GenBGMRequest {
	body := &GenBGMRequest{
		Text: req.Prompt,
	}
	if d := req.Duration; d > 0 {
		body.Duration = &d
	}
	if req.Metadata == nil {
		return body
	}

	body.TosBucket = stringFromMeta(req.Metadata, "tos_bucket")
	body.CallbackURL = stringFromMeta(req.Metadata, "callback_url")
	body.Version = stringFromMeta(req.Metadata, "version")

	if v, ok := boolFromMeta(req.Metadata, "enable_input_rewrite"); ok {
		body.EnableInputRewrite = &v
	}
	if v, ok := boolFromMeta(req.Metadata, "implicit_watermark"); ok {
		body.ImplicitWaterMark = &v
	}
	if seg, ok := req.Metadata["segments"]; ok {
		body.Segments = seg
	}
	if body.Duration == nil {
		if v, ok := intFromMeta(req.Metadata, "duration"); ok {
			body.Duration = &v
		}
	}
	return body
}

func stringFromMeta(meta map[string]any, key string) string {
	if v, ok := meta[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func intFromMeta(meta map[string]any, key string) (int, bool) {
	v, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
		}
	}
	return 0, false
}

func boolFromMeta(meta map[string]any, key string) (bool, bool) {
	v, ok := meta[key]
	if !ok {
		return false, false
	}
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		switch strings.ToLower(strings.TrimSpace(b)) {
		case "true":
			return true, true
		case "false":
			return false, true
		}
	}
	return false, false
}
