package service

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v4/client"
	"github.com/alibabacloud-go/tea/tea"
)

// SendSMSCode 通过阿里云短信服务发送验证码
func SendSMSCode(phone, code string) error {
	settings := system_setting.GetAliyunSMSSettings()
	if !settings.Enabled {
		return fmt.Errorf("短信服务未启用")
	}

	config := &openapi.Config{
		AccessKeyId:     tea.String(settings.AccessKeyId),
		AccessKeySecret: tea.String(settings.AccessSecret),
		Endpoint:        tea.String(settings.Endpoint),
	}

	client, err := dysmsapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("创建短信客户端失败: %w", err)
	}

	req := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(phone),
		SignName:      tea.String(settings.SignName),
		TemplateCode:  tea.String(settings.TemplateCode),
		TemplateParam: tea.String(fmt.Sprintf(`{"code":"%s"}`, code)),
	}

	resp, err := client.SendSms(req)
	if err != nil {
		common.SysError(fmt.Sprintf("发送短信失败: %s", err.Error()))
		return fmt.Errorf("发送短信失败: %w", err)
	}

	if resp.Body != nil && resp.Body.Code != nil && *resp.Body.Code != "OK" {
		msg := ""
		if resp.Body.Message != nil {
			msg = *resp.Body.Message
		}
		common.SysError(fmt.Sprintf("短信服务返回错误: code=%s, message=%s", *resp.Body.Code, msg))
		return fmt.Errorf("短信发送失败: %s", msg)
	}

	return nil
}
