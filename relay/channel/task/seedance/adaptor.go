package seedance

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
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================
// Response structures
// ============================

// submitResponse 提交任务的响应。
// 上游返回扁平结构：{"id":"task_xxx","task_id":"task_xxx","object":"video",...}
type submitResponse struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
}

func (s *submitResponse) GetTaskID() string {
	if s.TaskID != "" {
		return s.TaskID
	}
	return s.ID
}

// fetchResponse 查询任务的完整响应。
// 上游也是一个 new-api 实例，返回数据有多层嵌套：
//
//	{"code":"success","data":{"status":"SUCCESS","data":{...真正任务数据...}}}
type fetchResponse struct {
	Code string `json:"code"`
	Data struct {
		TaskID    string     `json:"task_id"`
		Status    string     `json:"status"`
		ResultURL string     `json:"result_url"`
		Data      taskResult `json:"data"`
	} `json:"data"`
}

// taskResult 上游真正的任务数据（VolcEngine Seedance 的响应）
type taskResult struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Duration   int    `json:"duration"`
	Resolution string `json:"resolution"`
	Usage      struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// EstimateBilling 计费估算，返回影响最终额度的 OtherRatios：
//   - resolution: 分辨率系数（480p=1.0, 720p=2.0, 1080p=4.0）
//   - video_input: 视频输入加价系数（当 content 包含 video_url 时生效）
//   - seconds: 仅 ModelPrice 模式下返回，用于 按秒 × 单价 预扣
//
// ModelPrice（按次计费）模式：预扣 = 单价 × 秒数 × 分辨率系数 × 视频输入系数
// ModelRatio（按 token 计费）模式：预扣一半 ModelRatio；
//
//	任务完成后框架按 total_tokens × ModelRatio × GroupRatio × (分辨率系数 × 视频输入系数) 差额结算
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	ratios := map[string]float64{}

	resolution := resolveResolution(c)
	if ratio, ok := GetResolutionRatio(resolution); ok && ratio > 0 && ratio != 1.0 {
		ratios["resolution"] = ratio
	}

	if hasVideoInput(c) {
		if ratio, ok := GetVideoInputRatio(info.OriginModelName); ok && ratio > 0 && ratio != 1.0 {
			ratios["video_input"] = ratio
		}
	}

	if info.PriceData.UsePrice {
		req, err := relaycommon.GetTaskRequest(c)
		if err == nil {
			seconds := resolveDuration(req)
			if seconds <= 0 {
				seconds = 4
			}
			ratios["seconds"] = float64(seconds)
		}
	}

	return ratios
}

// hasVideoInput 检查请求体的 content[] 中是否包含 video_url 类型的条目。
func hasVideoInput(c *gin.Context) bool {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return false
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return false
	}
	var bodyMap map[string]interface{}
	if err := common.Unmarshal(cachedBody, &bodyMap); err != nil {
		return false
	}
	contentRaw, ok := bodyMap["content"]
	if !ok {
		return false
	}
	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return false
	}
	for _, item := range contentSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemMap["type"] == "video_url" {
			return true
		}
	}
	return false
}

// resolveDuration 从 TaskSubmitReq 中解析视频时长（秒）。
// 优先级：顶层 duration(int) > 顶层 seconds(string) > metadata.duration。
func resolveDuration(req relaycommon.TaskSubmitReq) int {
	if req.Duration > 0 {
		return req.Duration
	}
	if sec, _ := strconv.Atoi(strings.TrimSpace(req.Seconds)); sec > 0 {
		return sec
	}
	if req.Metadata != nil {
		if raw, ok := req.Metadata["duration"]; ok {
			switch v := raw.(type) {
			case float64:
				return int(v)
			case int:
				return v
			case string:
				if sec, _ := strconv.Atoi(strings.TrimSpace(v)); sec > 0 {
					return sec
				}
			}
		}
	}
	return 0
}

// resolveResolution 从原始请求体中读取 resolution 字段。
// TaskSubmitReq 不含 resolution 字段，需从缓存 body 中解析。
func resolveResolution(c *gin.Context) string {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return ""
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return ""
	}
	var bodyMap map[string]interface{}
	if err := common.Unmarshal(cachedBody, &bodyMap); err != nil {
		return ""
	}
	if res, ok := bodyMap["resolution"].(string); ok {
		return strings.TrimSpace(res)
	}
	return ""
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	if strings.TrimSpace(req.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field model is required"), "invalid_request", http.StatusBadRequest)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = extractPromptFromMetadata(req.Metadata)
	}
	req.Prompt = prompt

	if len(req.Images) == 0 {
		req.Images = extractImageURLsFromMetadata(req.Metadata)
	}

	info.Action = constant.TaskActionGenerate
	info.OriginModelName = req.Model
	c.Set("task_request", req)
	return nil
}

// extractPromptFromMetadata reads the prompt from a content[] array stored in
// metadata (the upstream "content" payload). Returns "" when no text item is found.
func extractPromptFromMetadata(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return ""
	}
	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return ""
	}
	for _, item := range contentSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemMap["type"] == "text" {
			if text, ok := itemMap["text"].(string); ok && strings.TrimSpace(text) != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

// extractImageURLsFromMetadata collects image URLs from a content[] array in
// metadata so that downstream HasImage() / billing logic works for i2v inputs.
func extractImageURLsFromMetadata(metadata map[string]interface{}) []string {
	if metadata == nil {
		return nil
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return nil
	}
	contentSlice, ok := contentRaw.([]interface{})
	if !ok {
		return nil
	}
	var images []string
	for _, item := range contentSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if itemMap["type"] != "image_url" {
			continue
		}
		mediaURL, ok := itemMap["image_url"].(map[string]interface{})
		if !ok {
			continue
		}
		if url, ok := mediaURL["url"].(string); ok && url != "" {
			images = append(images, url)
		}
	}
	return images
}

// isVolcEngine 判断 Base URL 是否指向火山方舟官方 API。
func isVolcEngine(baseURL string) bool {
	return strings.Contains(baseURL, "volces.com") || strings.Contains(baseURL, "volcengine.com")
}

// submitPath 返回提交任务的 API 路径。
func submitPath(baseURL string) string {
	if isVolcEngine(baseURL) {
		return "/api/v3/contents/generations/tasks"
	}
	return "/v1/video/generations"
}

// fetchPath 返回查询任务的 API 路径。
func fetchPath(baseURL, taskID string) string {
	if isVolcEngine(baseURL) {
		return fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseURL, taskID)
	}
	return fmt.Sprintf("%s/v1/video/generations/%s", baseURL, taskID)
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s%s", a.baseURL, submitPath(a.baseURL)), nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}

	// Pass the upstream request body through as-is, only overriding the model
	// field when the channel has a model mapping configured.
	var bodyMap map[string]interface{}
	if err := common.Unmarshal(cachedBody, &bodyMap); err != nil {
		return nil, errors.Wrap(err, "unmarshal_request_body_failed")
	}

	if info.IsModelMapped {
		bodyMap["model"] = info.UpstreamModelName
	} else if modelStr, ok := bodyMap["model"].(string); ok && modelStr != "" {
		info.UpstreamModelName = modelStr
	}

	data, err := common.Marshal(bodyMap)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_request_body_failed")
	}
	common.SysLog(fmt.Sprintf("[seedance] upstream request body: %s", string(data)))
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	common.SysLog(fmt.Sprintf("[seedance] submit response body: %s", strings.TrimSpace(string(responseBody))))

	if resp.StatusCode != http.StatusOK {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("upstream returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody))),
			"upstream_error",
			resp.StatusCode,
		)
		return
	}

	var dResp submitResponse
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamTaskID := dResp.GetTaskID()
	if upstreamTaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty in upstream response, body: %s", strings.TrimSpace(string(responseBody))), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return upstreamTaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fetchPath(baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	common.SysLog(fmt.Sprintf("[seedance] fetch response body: %s", strings.TrimSpace(string(respBody))))

	inner, err := extractTaskResult(respBody)
	if err != nil {
		common.SysLog(fmt.Sprintf("[seedance] extractTaskResult failed: %s", err.Error()))
		return nil, err
	}

	common.SysLog(fmt.Sprintf("[seedance] extracted task: id=%s, status=%s, total_tokens=%d, completion_tokens=%d, video_url=%s",
		inner.ID, inner.Status, inner.Usage.TotalTokens, inner.Usage.CompletionTokens, inner.Content.VideoURL))

	result := relaycommon.TaskInfo{Code: 0}

	switch strings.ToLower(inner.Status) {
	case "queued", "pending":
		result.Status = model.TaskStatusQueued
		result.Progress = "10%"
	case "processing", "running":
		result.Status = model.TaskStatusInProgress
		result.Progress = "50%"
	case "succeeded", "completed", "success":
		result.Status = model.TaskStatusSuccess
		result.Progress = "100%"
		result.Url = inner.Content.VideoURL
		result.CompletionTokens = inner.Usage.CompletionTokens
		result.TotalTokens = inner.Usage.TotalTokens
	case "failed":
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
		result.Reason = inner.Error.Message
		if result.Reason == "" {
			result.Reason = inner.Error.Code
		}
	default:
		result.Status = model.TaskStatusInProgress
		result.Progress = "30%"
	}

	return &result, nil
}

// extractTaskResult 从响应体中提取真正的任务数据。
// 支持三种格式：
//  1. 火山方舟原生（扁平）：{"id":"cgt-xxx","status":"succeeded","usage":{...}}
//  2. new-api 中转（2层嵌套）：{"code":"success","data":{"status":"SUCCESS","data":{...}}}
//  3. 三方中转（3层嵌套）：{"code":"success","data":{"data":{"data":{...}}}}
//
// 递归向下查找含有 usage 字段的最深层对象。
func extractTaskResult(respBody []byte) (taskResult, error) {
	var raw map[string]interface{}
	if err := common.Unmarshal(respBody, &raw); err != nil {
		return taskResult{}, errors.Wrap(err, "unmarshal task result failed")
	}

	deepest := findDeepestTaskData(raw)
	if deepest == nil {
		return taskResult{}, errors.New("task data not found in response")
	}

	dataBytes, err := common.Marshal(deepest)
	if err != nil {
		return taskResult{}, errors.Wrap(err, "marshal task data failed")
	}

	var tr taskResult
	if err := common.Unmarshal(dataBytes, &tr); err != nil {
		return taskResult{}, errors.Wrap(err, "unmarshal task data failed")
	}
	return tr, nil
}

// findDeepestTaskData 递归查找含有 usage 字段的最深层 map。
// 如果当前 map 有 usage 字段，直接返回；
// 否则查找子 map 中是否有 data 字段继续递归。
func findDeepestTaskData(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	if _, hasUsage := m["usage"]; hasUsage {
		return m
	}
	for key, val := range m {
		if key == "data" {
			if subMap, ok := val.(map[string]interface{}); ok {
				if result := findDeepestTaskData(subMap); result != nil {
					return result
				}
			}
		}
	}
	return nil
}

// AdjustBillingOnComplete 返回 0，让框架走 token 重算路径：
// 任务完成后，框架按 total_tokens × ModelRatio × GroupRatio 自动计算实际额度，
// 与预扣额度做差额结算（多退少补）。
// 仅在 ModelRatio（非 ModelPrice）模式下生效，因为 ModelPrice 模式 PerCallBilling=true 会跳过此步。

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	inner, err := extractTaskResult(originTask.Data)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal seedance task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", inner.Content.VideoURL)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if strings.ToLower(inner.Status) == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: inner.Error.Message,
			Code:    inner.Error.Code,
		}
	}

	return common.Marshal(openAIVideo)
}
