package relay

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func AudioHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	audioReq, ok := info.Request.(*dto.AudioRequest)
	if !ok {
		return types.NewError(errors.New("invalid request type"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(audioReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to AudioRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	ioReader, err := adaptor.ConvertAudioRequest(c, info, *request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	resp, err := adaptor.DoRequest(c, info, ioReader)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	statusCodeMappingStr := c.GetString("status_code_mapping")

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	// 检查是否是豆包语音，并且是否有时长信息
	if info.ApiType == constant.APITypeDoubaoVoice {
		if durationRaw, exists := c.Get("doubao_audio_duration"); exists {
			if durationSeconds, ok := durationRaw.(float64); ok && durationSeconds > 0 {
				// 使用时长计费
				// 1元/分钟
				const yuanPerMinute = 1.0
				minutes := durationSeconds / 60.0
				amountYuan := minutes * yuanPerMinute
				quotaPerUnit := float64(common.QuotaPerUnit)
				quota := int(amountYuan * quotaPerUnit)

				// 结算quota
				if err := service.SettleBilling(c, info, quota); err != nil {
					// 如果结算失败，记录日志但不影响响应
					logger.LogError(c, fmt.Sprintf("Failed to settle billing for doubao voice: %v", err))
				}

				// 记录日志
				other := map[string]any{
					"audio_duration_seconds": durationSeconds,
					"audio_duration_minutes": minutes,
					"price_yuan":             amountYuan,
					"quota":                  quota,
				}

				model.RecordConsumeLog(c, info.UserId, model.RecordConsumeLogParams{
					ChannelId: info.ChannelId,
					ModelName: info.OriginModelName,
					TokenName: c.GetString("token_name"),
					Quota:     quota,
					Content:   fmt.Sprintf("豆包语音生成，时长 %.2f 秒，费用 %.2f 元", durationSeconds, amountYuan),
					TokenId:   info.TokenId,
					Group:     info.UsingGroup,
					Other:     other,
				})

				model.UpdateUserUsedQuotaAndRequestCount(info.UserId, quota)
				model.UpdateChannelUsedQuota(info.ChannelId, quota)

				return nil
			}
		}
	}

	// 对于其他情况，使用正常的计费方式
	if usage.(*dto.Usage).CompletionTokenDetails.AudioTokens > 0 || usage.(*dto.Usage).PromptTokensDetails.AudioTokens > 0 {
		service.PostAudioConsumeQuota(c, info, usage.(*dto.Usage), "")
	} else {
		service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
	}

	return nil
}
