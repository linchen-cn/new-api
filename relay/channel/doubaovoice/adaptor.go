package doubaovoice

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	"github.com/QuantumNous/new-api/service"
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
		baseUrl = channelconstant.ChannelBaseURLs[channelconstant.ChannelTypeDoubaoVoice]
	}
	return fmt.Sprintf("%s/api/v3/tts/create", baseUrl), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	req.Set("Content-Type", "application/json")

	// 支持两种鉴权方式：
	// 1. 简单的 X-Api-Key 方式
	// 2. 火山引擎标准的 appID|token 方式（与现有的火山引擎适配器保持一致）
	apiKey := info.ApiKey
	if strings.Contains(apiKey, "|") {
		// 如果包含 |，则解析为 appID 和 token
		parts := strings.SplitN(apiKey, "|", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			// 使用火山引擎的双头鉴权方式
			req.Set("X-Api-App-Id", parts[0])
			req.Set("X-Api-Key", parts[1])
			return nil
		}
	}

	// 否则，使用简单的 X-Api-Key 方式
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

var responseFormatToEncodingMap = map[string]string{
	"mp3":  "mp3",
	"opus": "ogg_opus",
	"aac":  "mp3",
	"flac": "mp3",
	"wav":  "wav",
	"pcm":  "pcm",
}

func mapEncoding(responseFormat string) string {
	if encoding, ok := responseFormatToEncodingMap[responseFormat]; ok {
		return encoding
	}
	return "wav"
}

func getContentTypeByEncoding(encoding string) string {
	contentTypeMap := map[string]string{
		"mp3":      "audio/mpeg",
		"ogg_opus": "audio/ogg",
		"wav":      "audio/wav",
		"pcm":      "audio/pcm",
	}
	if ct, ok := contentTypeMap[encoding]; ok {
		return ct
	}
	return "application/octet-stream"
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != constant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}

	// 初始化基础请求
	doubaoReq := DoubaoVoiceRequest{
		Model:        info.UpstreamModelName,
		TextPrompt:   request.Input,
	}

	// 设置 speaker 或根据 voice 字段
	if request.Voice != "" {
		doubaoReq.Speaker = request.Voice
	}

	// 设置基础音频配置
	audioConfig := AudioConfig{}
	if request.ResponseFormat != "" {
		audioConfig.Format = mapEncoding(request.ResponseFormat)
	}
	if request.Speed != nil {
		// 将 OpenAI 的速度（0.25-4.0）转换为豆包语音的 speech_rate (-50 到 100)
		// 1.0 对应 0，0.25 对应 -50，4.0 对应 100
		speedRatio := *request.Speed
		if speedRatio <= 0.25 {
			audioConfig.SpeechRate = -50
		} else if speedRatio >= 4.0 {
			audioConfig.SpeechRate = 100
		} else {
			audioConfig.SpeechRate = int((speedRatio - 1.0) * 100 / 3.0)
		}
	}
	doubaoReq.AudioConfig = audioConfig

	// 如果有 metadata 字段，我们可以合并所有豆包语音特有的参数
	if len(request.Metadata) > 0 {
		var metadataMap map[string]any
		if err := json.Unmarshal(request.Metadata, metadataMap); err == nil {
			// 优先检查 metadata 中是否有完整的 doubao_request 对象，如果有就直接使用
			if fullReq, ok := metadataMap["doubao_request"].(map[string]any); ok {
				// 将完整的请求对象转换为 JSON 再解析到我们的结构体中
				fullReqJson, jsonErr := json.Marshal(fullReq)
				if jsonErr == nil {
					var fullDoubaoReq DoubaoVoiceRequest
					if parseErr := json.Unmarshal(fullReqJson, &fullDoubaoReq); parseErr == nil {
						// 保留我们已经设置的必要字段（model，text_prompt等）
						if fullDoubaoReq.Model == "" {
							fullDoubaoReq.Model = info.UpstreamModelName
						}
						if fullDoubaoReq.TextPrompt == "" {
							fullDoubaoReq.TextPrompt = request.Input
						}
						doubaoReq = fullDoubaoReq
					}
				}
			} else {
				// 否则，逐个合并字段
				if speaker, ok := metadataMap["speaker"].(string); ok {
					doubaoReq.Speaker = speaker
				}
				if audioData, ok := metadataMap["audio_data"].(string); ok {
					doubaoReq.AudioData = audioData
				}
				if audioUrl, ok := metadataMap["audio_url"].(string); ok {
					doubaoReq.AudioUrl = audioUrl
				}
				if imageData, ok := metadataMap["image_data"].(string); ok {
					doubaoReq.ImageData = imageData
				}
				if imageUrl, ok := metadataMap["image_url"].(string); ok {
					doubaoReq.ImageUrl = imageUrl
				}
				// 处理 references
				if referencesRaw, ok := metadataMap["references"].([]any); ok {
					var references []Reference
					for _, refRaw := range referencesRaw {
						if refMap, ok := refRaw.(map[string]any); ok {
							var ref Reference
							if speaker, ok := refMap["speaker"].(string); ok {ref.Speaker = speaker}
							if audioData, ok := refMap["audio_data"].(string); ok {ref.AudioData = audioData}
							if audioUrl, ok := refMap["audio_url"].(string); ok {ref.AudioUrl = audioUrl}
							if imageData, ok := refMap["image_data"].(string); ok {ref.ImageData = imageData}
							if imageUrl, ok := refMap["image_url"].(string); ok {ref.ImageUrl = imageUrl}
							references = append(references, ref)
						}
					}
					doubaoReq.References = references
				}
				// 处理 audio_config
				if audioConfigMap, ok := metadataMap["audio_config"].(map[string]any); ok {
					if format, ok := audioConfigMap["format"].(string); ok {
						doubaoReq.AudioConfig.Format = format
					}
					if sampleRate, ok := audioConfigMap["sample_rate"].(float64); ok {
						doubaoReq.AudioConfig.SampleRate = int(sampleRate)
					}
					if speechRate, ok := audioConfigMap["speech_rate"].(float64); ok {
						doubaoReq.AudioConfig.SpeechRate = int(speechRate)
					}
					if loudnessRate, ok := audioConfigMap["loudness_rate"].(float64); ok {
						doubaoReq.AudioConfig.LoudnessRate = int(loudnessRate)
					}
					if pitchRate, ok := audioConfigMap["pitch_rate"].(float64); ok {
						doubaoReq.AudioConfig.PitchRate = int(pitchRate)
					}
					if enableSubtitle, ok := audioConfigMap["enable_subtitle"].(bool); ok {
						doubaoReq.AudioConfig.EnableSubtitle = enableSubtitle
					}
				}
			}
		}
	}

	// 保存 audio config 到 context，用于响应处理
	c.Set("doubao_audio_config", doubaoReq.AudioConfig)

	jsonData, err := json.Marshal(doubaoReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling doubao voice request: %w", err)
	}

	return bytes.NewReader(jsonData), nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("failed to read doubao voice response"),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}
	defer resp.Body.Close()

	var doubaoResp DoubaoVoiceResponse
	if unmarshalErr := json.Unmarshal(body, &doubaoResp); unmarshalErr != nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("failed to parse doubao voice response"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	if doubaoResp.Code != 0 {
		// 提供更详细的错误信息
		errorMsg := fmt.Sprintf("doubao voice error: code=%d, message=%s", doubaoResp.Code, doubaoResp.Message)
		return nil, types.NewErrorWithStatusCode(
			errors.New(errorMsg),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}

	// 保存语音时长到 context，用于后续计费
	if doubaoResp.OriginalDuration > 0 {
		c.Set("doubao_audio_duration", doubaoResp.OriginalDuration)
	} else if doubaoResp.Duration > 0 {
		c.Set("doubao_audio_duration", doubaoResp.Duration)
	}

	// 解码 Base64 音频数据
	audioData, decodeErr := base64.StdEncoding.DecodeString(doubaoResp.Audio)
	if decodeErr != nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("failed to decode audio data"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	// 确定内容类型
	audioConfig, _ := c.Get("doubao_audio_config")
	encoding := "wav"
	if ac, ok := audioConfig.(AudioConfig); ok && ac.Format != "" {
		encoding = ac.Format
	}
	contentType := getContentTypeByEncoding(encoding)
	c.Header("Content-Type", contentType)
	c.Data(http.StatusOK, contentType, audioData)

	usage = &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      info.GetEstimatePromptTokens(),
	}

	return usage, nil
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// RecalculateDoubaoVoiceQuota 根据语音时长重新计算quota
func RecalculateDoubaoVoiceQuota(c *gin.Context, info *relaycommon.RelayInfo, durationSeconds float64) error {
	// 1元/分钟，所以每秒钟是 1/60 元
	const yuanPerMinute = 1.0
	seconds := durationSeconds
	minutes := seconds / 60.0

	// 计算所需金额
	amountYuan := minutes * yuanPerMinute

	// 转换为配额单位，1元等于 common.QuotaPerUnit 个单位
	quotaPerUnit := float64(common.QuotaPerUnit)
	quota := int(amountYuan * quotaPerUnit)

	// 调用 SettleBilling 进行差额结算
	if err := service.SettleBilling(c, info, quota); err != nil {
		return err
	}

	return nil
}
