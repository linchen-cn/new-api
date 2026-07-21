package doubaotts

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := info.ChannelBaseUrl
	if baseUrl == "" {
		baseUrl = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeDoubaoTTS]
	}
	return fmt.Sprintf("%s/api/v3/tts/unidirectional", baseUrl), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	req.Set("Content-Type", "application/json")
	req.Set("Connection", "keep-alive")

	// 设置 X-Api-Resource-Id 头，根据模型名映射到对应的 resource id
	resourceID := GetResourceID(info.UpstreamModelName)
	req.Set("X-Api-Resource-Id", resourceID)

	// 鉴权：支持简单的 X-Api-Key 方式和 appID|token 双头方式
	apiKey := info.ApiKey
	if strings.Contains(apiKey, "|") {
		parts := strings.SplitN(apiKey, "|", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			req.Set("X-Api-App-Id", parts[0])
			req.Set("X-Api-Key", parts[1])
			return nil
		}
	}

	req.Set("X-Api-Key", apiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

var responseFormatMap = map[string]string{
	"mp3":  "mp3",
	"opus": "ogg_opus",
	"aac":  "mp3",
	"flac": "mp3",
	"wav":  "wav",
	"pcm":  "pcm",
}

func mapFormat(responseFormat string) string {
	if format, ok := responseFormatMap[responseFormat]; ok {
		return format
	}
	return "wav"
}

func getContentType(format string) string {
	contentTypeMap := map[string]string{
		"mp3":      "audio/mpeg",
		"ogg_opus": "audio/ogg",
		"wav":      "audio/wav",
		"pcm":      "audio/pcm",
	}
	if ct, ok := contentTypeMap[format]; ok {
		return ct
	}
	return "application/octet-stream"
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}

	// 构建豆包 TTS 请求
	reqParams := ReqParams{
		Text: request.Input,
	}

	// 设置 speaker（对应 OpenAI 的 voice 字段）
	if request.Voice != "" {
		reqParams.Speaker = request.Voice
	}

	// 设置音频参数
	audioParams := AudioParams{}
	if request.ResponseFormat != "" {
		audioParams.Format = mapFormat(request.ResponseFormat)
	}

	// 通过 metadata 合并豆包 TTS 特有参数
	if len(request.Metadata) > 0 {
		var metadataMap map[string]any
		if err := common.Unmarshal(request.Metadata, &metadataMap); err == nil {
			// 检查是否有完整的 doubao_tts_request 对象
			if fullReq, ok := metadataMap["doubao_tts_request"].(map[string]any); ok {
				fullReqJson, jsonErr := common.Marshal(fullReq)
				if jsonErr == nil {
					var fullDoubaoReq DoubaoTTSRequest
					if parseErr := common.Unmarshal(fullReqJson, &fullDoubaoReq); parseErr == nil {
						if fullDoubaoReq.ReqParams.Text == "" {
							fullDoubaoReq.ReqParams.Text = request.Input
						}
						c.Set("doubaotts_audio_params", fullDoubaoReq.ReqParams.AudioParams)
						jsonData, err := common.Marshal(fullDoubaoReq)
						if err != nil {
							return nil, fmt.Errorf("error marshalling doubao tts request: %w", err)
						}
						return bytes.NewReader(jsonData), nil
					}
				}
			}

			// 逐个合并字段
			if speaker, ok := metadataMap["speaker"].(string); ok {
				reqParams.Speaker = speaker
			}
			if text, ok := metadataMap["text"].(string); ok && text != "" {
				reqParams.Text = text
			}
			if audioParamsMap, ok := metadataMap["audio_params"].(map[string]any); ok {
				if format, ok := audioParamsMap["format"].(string); ok {
					audioParams.Format = format
				}
				if sampleRate, ok := audioParamsMap["sample_rate"].(float64); ok {
					audioParams.SampleRate = int(sampleRate)
				}
			}
		}
	}

	reqParams.AudioParams = audioParams
	c.Set("doubaotts_audio_params", audioParams)

	doubaoReq := DoubaoTTSRequest{
		ReqParams: reqParams,
	}

	jsonData, err := common.Marshal(doubaoReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling doubao tts request: %w", err)
	}

	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()

	// /api/v3/tts/unidirectional 返回流式 JSON 响应（每行一个 JSON 对象）
	// 需要逐行读取，解码每个 base64 音频块，并拼接在一起
	scanner := bufio.NewScanner(resp.Body)
	// 增大 buffer 以支持较长的 base64 行
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var audioBuffer bytes.Buffer
	var totalWords int

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk DoubaoTTSResponse
		if unmarshalErr := common.Unmarshal(line, &chunk); unmarshalErr != nil {
			// 跳过无法解析的行（可能是空行或注释）
			continue
		}

		// 检查错误码：成功码为 0 或 20000000（message 为 "OK"）
		if chunk.Code != 0 && chunk.Code != 20000000 {
			errorMsg := fmt.Sprintf("doubao tts error: code=%d, message=%s", chunk.Code, chunk.Message)
			return nil, types.NewErrorWithStatusCode(
				errors.New(errorMsg),
				types.ErrorCodeBadResponse,
				http.StatusBadRequest,
			)
		}

		// 解码 base64 音频块并追加到 buffer
		if chunk.Data != "" {
			audioData, decodeErr := base64.StdEncoding.DecodeString(chunk.Data)
			if decodeErr != nil {
				return nil, types.NewErrorWithStatusCode(
					errors.New("failed to decode audio chunk"),
					types.ErrorCodeBadResponseBody,
					http.StatusInternalServerError,
				)
			}
			audioBuffer.Write(audioData)
		}

		// 累计 usage 信息
		if chunk.Usage.TextWords > 0 {
			totalWords = chunk.Usage.TextWords
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to read streaming response: %w", scanErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	// 确定内容类型
	audioParams, _ := c.Get("doubaotts_audio_params")
	format := "wav"
	if ap, ok := audioParams.(AudioParams); ok && ap.Format != "" {
		format = ap.Format
	}
	contentType := getContentType(format)
	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, audioBuffer.Bytes())

	// 使用 text_words 作为 prompt tokens，如果没有则使用估算值
	promptTokens := totalWords
	if promptTokens == 0 {
		promptTokens = info.GetEstimatePromptTokens()
	}
	usage = &dto.Usage{
		PromptTokens:    promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}

	return usage, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
