package system_setting

import "github.com/QuantumNous/new-api/setting/config"

type AliyunSMSSettings struct {
	Enabled      bool   `json:"enabled"`
	AccessKeyId  string `json:"access_key_id"`
	AccessSecret string `json:"access_secret"`
	SignName     string `json:"sign_name"`
	TemplateCode string `json:"template_code"`
	Endpoint    string `json:"endpoint"`
}

var defaultAliyunSMSSettings = AliyunSMSSettings{
	Endpoint: "dysmsapi.aliyuncs.com",
}

func init() {
	config.GlobalConfig.Register("aliyun_sms", &defaultAliyunSMSSettings)
}

func GetAliyunSMSSettings() *AliyunSMSSettings {
	return &defaultAliyunSMSSettings
}
