package controller

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// setupPlaygroundContext prepares the request context for a playground relay.
// It rejects access tokens, loads the user cache, and creates a temporary token
// so the standard relay/task pipeline can handle billing and distribution.
func setupPlaygroundContext(c *gin.Context, relayFormat types.RelayFormat) error {
	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		return errors.New("暂不支持使用 access token")
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		return err
	}

	userId := c.GetInt("id")

	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return err
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	return nil
}

func Playground(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	if err := setupPlaygroundContext(c, types.RelayFormatOpenAI); err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	Relay(c, types.RelayFormatOpenAI)
}

func PlaygroundImage(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	if err := setupPlaygroundContext(c, types.RelayFormatOpenAIImage); err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	Relay(c, types.RelayFormatOpenAIImage)
}

func PlaygroundAudio(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		if newAPIError != nil {
			c.JSON(newAPIError.StatusCode, gin.H{
				"error": newAPIError.ToOpenAIError(),
			})
		}
	}()

	if err := setupPlaygroundContext(c, types.RelayFormatOpenAIAudio); err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return
	}

	Relay(c, types.RelayFormatOpenAIAudio)
}

func PlaygroundVideo(c *gin.Context) {
	if err := setupPlaygroundContext(c, types.RelayFormatTask); err != nil {
		c.JSON(http.StatusForbidden, &dto.TaskError{
			Code:       "playground_access_denied",
			Message:    err.Error(),
			StatusCode: http.StatusForbidden,
		})
		return
	}

	RelayTask(c)
}

func PlaygroundVideoFetch(c *gin.Context) {
	if err := setupPlaygroundContext(c, types.RelayFormatTask); err != nil {
		c.JSON(http.StatusForbidden, &dto.TaskError{
			Code:       "playground_access_denied",
			Message:    err.Error(),
			StatusCode: http.StatusForbidden,
		})
		return
	}

	RelayTaskFetch(c)
}

// PlaygroundModels returns the user's available models with their supported
// endpoint types, allowing the frontend to filter models by capability
// (text, image, video).
func PlaygroundModels(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserCache(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups := service.GetUserUsableGroups(user.Group)

	type playgroundModel struct {
		Name                   string                    `json:"name"`
		SupportedEndpointTypes []constant.EndpointType   `json:"supported_endpoint_types"`
	}

	seen := make(map[string]bool)
	var models []playgroundModel
	for group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if seen[modelName] {
				continue
			}
			seen[modelName] = true
			models = append(models, playgroundModel{
				Name:                   modelName,
				SupportedEndpointTypes: model.GetModelSupportEndpointTypes(modelName),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}
