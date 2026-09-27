package seedance

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestContext(t *testing.T, body string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/pg/video/generations", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	// Pre-populate body storage so BuildRequestBody can read the cached body.
	_, err := common.GetRequestBody(c)
	require.NoError(t, err)
	return c
}

func decodeBody(t *testing.T, r interface{ Read([]byte) (int, error) }) map[string]interface{} {
	t.Helper()
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(r)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, common.Unmarshal(buf.Bytes(), &m))
	return m
}

func TestBuildRequestBodySynthesizesContentFromPromptOnly(t *testing.T) {
	body := `{"model":"Doubao-Seedance-2.0","group":"default","prompt":"生成一段中国古风的打斗场景","duration":4,"resolution":"480p"}`
	c := setupTestContext(t, body)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, OriginModelName: "Doubao-Seedance-2.0"}

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	m := decodeBody(t, reader)

	// group must be stripped
	_, hasGroup := m["group"]
	assert.False(t, hasGroup)
	// prompt/images/image must be stripped
	_, hasPrompt := m["prompt"]
	assert.False(t, hasPrompt)

	// content must be synthesized with a single text item
	content, ok := m["content"].([]interface{})
	require.True(t, ok)
	require.Len(t, content, 1)
	textItem, ok := content[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text", textItem["type"])
	assert.Equal(t, "生成一段中国古风的打斗场景", textItem["text"])

	assert.Equal(t, float64(4), m["duration"])
	assert.Equal(t, "480p", m["resolution"])
}

func TestBuildRequestBodySynthesizesContentFromPromptAndImages(t *testing.T) {
	body := `{"model":"Doubao-Seedance-2.0-fast","prompt":"一辆汽车乘风破浪","images":["https://a.example/1.jpg","https://a.example/2.jpg"],"duration":4}`
	c := setupTestContext(t, body)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, OriginModelName: "Doubao-Seedance-2.0-fast"}

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	m := decodeBody(t, reader)

	content, ok := m["content"].([]interface{})
	require.True(t, ok)
	require.Len(t, content, 3)

	first, ok := content[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "text", first["type"])

	for i := 1; i <= 2; i++ {
		img, ok := content[i].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "image_url", img["type"])
		assert.Equal(t, "reference_image", img["role"])
		_, ok = img["image_url"].(map[string]interface{})
		require.True(t, ok)
	}
	url1 := content[1].(map[string]interface{})["image_url"].(map[string]interface{})["url"]
	url2 := content[2].(map[string]interface{})["image_url"].(map[string]interface{})["url"]
	assert.Equal(t, "https://a.example/1.jpg", url1)
	assert.Equal(t, "https://a.example/2.jpg", url2)
}

func TestBuildRequestBodyPassesThroughExistingContent(t *testing.T) {
	body := `{"model":"Doubao-Seedance-2.0-fast","content":[
		{"type":"image_url","image_url":{"url":"https://a.example/first.jpg"},"role":"first_frame"},
		{"type":"text","text":"A cinematic reveal"},
		{"type":"image_url","image_url":{"url":"https://a.example/last.jpg"},"role":"last_frame"},
		{"type":"video_url","video_url":{"url":"https://a.example/ref.mp4"},"role":"reference_video"}
	],"duration":8,"resolution":"720p","ratio":"16:9","generate_audio":true}`
	c := setupTestContext(t, body)
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}, OriginModelName: "Doubao-Seedance-2.0-fast"}

	reader, err := (&TaskAdaptor{}).BuildRequestBody(c, info)
	require.NoError(t, err)
	m := decodeBody(t, reader)

	// content passes through unchanged (4 items with roles preserved)
	content, ok := m["content"].([]interface{})
	require.True(t, ok)
	require.Len(t, content, 4)

	first := content[0].(map[string]interface{})
	assert.Equal(t, "first_frame", first["role"])
	video := content[3].(map[string]interface{})
	assert.Equal(t, "video_url", video["type"])
	assert.Equal(t, "reference_video", video["role"])

	// other Seedance fields pass through
	assert.Equal(t, true, m["generate_audio"])
	assert.Equal(t, "16:9", m["ratio"])
}

func TestBuildSeedanceContentEmpty(t *testing.T) {
	assert.Empty(t, buildSeedanceContent(map[string]interface{}{}))
	assert.Empty(t, buildSeedanceContent(map[string]interface{}{"prompt": "   "}))
}

func TestParseTaskResultNativeVolcFormat(t *testing.T) {
	// 火山方舟原生格式:视频地址在 content.video_url,token 用量在 usage
	body := `{"id":"cgt-20260924","model":"Doubao-Seedance-2.0","status":"succeeded",` +
		`"content":{"video_url":"https://tos.example/native.mp4"},` +
		`"usage":{"completion_tokens":48400,"total_tokens":48400}}`

	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "https://tos.example/native.mp4", result.Url)
	assert.Equal(t, 48400, result.TotalTokens)
	assert.Equal(t, 48400, result.CompletionTokens)
}

func TestParseTaskResultFlatNewApiRelayFormat(t *testing.T) {
	// new-api 兼容中转的扁平格式:视频地址在顶层 url,无 usage
	// 回归用例:真实事故——适配器取不到顶层 url 导致结算拿不到视频地址,落入代理兜底
	body := `{"completed_at":1790222413,"created_at":1790222258,"id":"ptask-12473b8e",` +
		`"model":"doubao-seedance-2.0","progress":100,"status":"completed",` +
		`"url":"https://tos.example/flat.mp4"}`

	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "https://tos.example/flat.mp4", result.Url)
}

func TestParseTaskResultFlatWithTopLevelUsage(t *testing.T) {
	// 修复后 new-api 中转的扁平格式:顶层 url + 顶层 usage
	// 下游必须能同时拿到视频地址和 token 用量,否则轮询终态后无法差额结算
	body := `{"id":"task_abc","status":"completed","url":"https://tos.example/relay.mp4",` +
		`"usage":{"completion_tokens":50638,"total_tokens":50638}}`

	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, model.TaskStatusSuccess, result.Status)
	assert.Equal(t, "https://tos.example/relay.mp4", result.Url)
	assert.Equal(t, 50638, result.TotalTokens)
	assert.Equal(t, 50638, result.CompletionTokens)
}

func TestConvertToOpenAIVideoPassesUsageAndURL(t *testing.T) {
	// new-api 作为中转时的输出契约:必须透传 usage,下游才能按 token 重算并差额结算(多退少补)
	data := `{"id":"cgt-1","status":"succeeded","content":{"video_url":"https://tos.example/native.mp4"},` +
		`"usage":{"completion_tokens":48400,"total_tokens":48400}}`
	task := &model.Task{
		TaskID:     "task_abc",
		Status:     model.TaskStatusSuccess,
		Data:       []byte(data),
		Properties: model.Properties{OriginModelName: "Doubao-Seedance-2.0"},
	}

	out, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var ov dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(out, &ov))
	require.NotNil(t, ov.Usage)
	assert.Equal(t, 48400, ov.Usage.TotalTokens)
	assert.Equal(t, 48400, ov.Usage.CompletionTokens)
	url, ok := ov.Metadata["url"].(string)
	require.True(t, ok)
	assert.Equal(t, "https://tos.example/native.mp4", url)
}
