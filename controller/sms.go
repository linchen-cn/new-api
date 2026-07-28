package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

type SendSMSCodeRequest struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

// SendSMSCode 发送短信验证码
func SendSMSCode(c *gin.Context) {
	settings := system_setting.GetAliyunSMSSettings()
	if !settings.Enabled {
		common.ApiErrorI18n(c, i18n.MsgSMSNotEnabled)
		return
	}

	var req SendSMSCodeRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	phone := strings.TrimSpace(req.Phone)
	if !phoneRegex.MatchString(phone) {
		common.ApiErrorI18n(c, i18n.MsgSMSInvalidPhone)
		return
	}

	purpose := common.SMSLoginPurpose
	phoneTaken := model.IsPhoneAlreadyTaken(phone)

	if req.Purpose == "bind" {
		purpose = common.SMSBindPurpose
		if phoneTaken {
			common.ApiErrorI18n(c, i18n.MsgSMSPhoneAlreadyBound)
			return
		}
	} else {
		if !phoneTaken {
			common.ApiErrorI18n(c, i18n.MsgSMSPhoneNotRegistered)
			return
		}
	}

	code := common.GenerateNumericVerificationCode(6)
	common.RegisterVerificationCodeWithKey(phone, code, purpose)
	common.SysLog(fmt.Sprintf("[sms] verification code sent: phone=%s, purpose=%s, code=%s", phone, purpose, code))

	if err := service.SendSMSCode(phone, code); err != nil {
		common.DeleteKey(phone, purpose)
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": i18n.T(c, i18n.MsgSMSSendFailed) + ": " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgSMSCodeSent),
	})
}

type SMSLoginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// SMSLogin 手机号验证码登录
func SMSLogin(c *gin.Context) {
	if !system_setting.GetAliyunSMSSettings().Enabled {
		common.ApiErrorI18n(c, i18n.MsgSMSLoginNotEnabled)
		return
	}

	var req SMSLoginRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.Code)

	if phone == "" || code == "" {
		common.ApiErrorI18n(c, i18n.MsgSMSPhoneOrCodeEmpty)
		return
	}

	if !phoneRegex.MatchString(phone) {
		common.ApiErrorI18n(c, i18n.MsgSMSInvalidPhone)
		return
	}

	if !common.VerifyCodeWithKey(phone, code, common.SMSLoginPurpose) {
		common.ApiErrorI18n(c, i18n.MsgSMSCodeError)
		return
	}

	common.DeleteKey(phone, common.SMSLoginPurpose)

	user := model.User{Phone: phone}
	if err := user.FillUserByPhone(); err != nil {
		common.ApiErrorI18n(c, i18n.MsgSMSPhoneNotRegistered)
		return
	}

	if user.Status != common.UserStatusEnabled {
		common.ApiErrorI18n(c, i18n.MsgSMSUserDisabled)
		return
	}

	setupLogin(&user, c)
}

// BindPhone 绑定手机号（需登录）
func BindPhone(c *gin.Context) {
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.Code)

	if !phoneRegex.MatchString(phone) || code == "" {
		common.ApiErrorI18n(c, i18n.MsgSMSPhoneOrCodeEmpty)
		return
	}

	if !common.VerifyCodeWithKey(phone, code, common.SMSBindPurpose) {
		common.ApiErrorI18n(c, i18n.MsgSMSCodeError)
		return
	}
	common.DeleteKey(phone, common.SMSBindPurpose)

	userId := c.GetInt("id")
	if model.IsPhoneAlreadyTaken(phone) {
		common.ApiErrorI18n(c, i18n.MsgSMSPhoneAlreadyBound)
		return
	}

	err := model.DB.Model(&model.User{}).Where("id = ?", userId).Update("phone", phone).Error
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": i18n.T(c, i18n.MsgSMSBindSuccess),
	})
}
