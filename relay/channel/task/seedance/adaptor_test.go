package seedance

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
