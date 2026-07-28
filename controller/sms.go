package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "短信服务未启用",
		})
		return
	}

	var req SendSMSCodeRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	phone := strings.TrimSpace(req.Phone)
	if !phoneRegex.MatchString(phone) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "手机号格式不正确",
		})
		return
	}

	purpose := common.SMSLoginPurpose
	phoneTaken := model.IsPhoneAlreadyTaken(phone)

	if req.Purpose == "bind" {
		purpose = common.SMSBindPurpose
		if phoneTaken {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "该手机号已被其他账号绑定",
			})
			return
		}
	} else {
		if !phoneTaken {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "该手机号未注册",
			})
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
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "验证码已发送",
	})
}

type SMSLoginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// SMSLogin 手机号验证码登录
func SMSLogin(c *gin.Context) {
	if !system_setting.GetAliyunSMSSettings().Enabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "短信登录未启用",
		})
		return
	}

	var req SMSLoginRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.Code)

	if phone == "" || code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "手机号和验证码不能为空",
		})
		return
	}

	if !phoneRegex.MatchString(phone) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "手机号格式不正确",
		})
		return
	}

	if !common.VerifyCodeWithKey(phone, code, common.SMSLoginPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码错误或已过期",
		})
		return
	}

	common.DeleteKey(phone, common.SMSLoginPurpose)

	user := model.User{Phone: phone}
	if err := user.FillUserByPhone(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该手机号未注册",
		})
		return
	}

	if user.Status != common.UserStatusEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已被禁用",
		})
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
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "无效的请求参数",
		})
		return
	}

	phone := strings.TrimSpace(req.Phone)
	code := strings.TrimSpace(req.Code)

	if !phoneRegex.MatchString(phone) || code == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "手机号或验证码无效",
		})
		return
	}

	if !common.VerifyCodeWithKey(phone, code, common.SMSBindPurpose) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "验证码错误或已过期",
		})
		return
	}
	common.DeleteKey(phone, common.SMSBindPurpose)

	userId := c.GetInt("id")
	if model.IsPhoneAlreadyTaken(phone) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该手机号已被其他账号绑定",
		})
		return
	}

	err := model.DB.Model(&model.User{}).Where("id = ?", userId).Update("phone", phone).Error
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "手机号绑定成功",
	})
}
