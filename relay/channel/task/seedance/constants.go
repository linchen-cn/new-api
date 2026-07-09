package seedance

var ModelList = []string{
	"Doubao-Seedance-2.0",
	"Doubao-Seedance-2.0-fast",
}

var ChannelName = "Seedance"

// resolutionRatioMap 分辨率计费系数（相对于 480p 的倍数）。
// 管理员将 ModelRatio/ModelPrice 配置为 480p 的基础单价，
// 其他分辨率会自动乘以此系数。
var resolutionRatioMap = map[string]float64{
	"480p":  1.0,
	"720p":  1.0,
	"1080p": 1.1087,
}

// GetResolutionRatio 返回指定分辨率的计费倍数。
func GetResolutionRatio(resolution string) (float64, bool) {
	r, ok := resolutionRatioMap[resolution]
	return r, ok
}

// videoInputRatioMap 视频输入计费系数。
// 当请求中包含视频输入（content[].type == "video_url"）时，
// 按模型名查找系数并乘到最终额度上。
// 管理员可在此为不同模型配置不同的视频输入加价倍数。
var videoInputRatioMap = map[string]float64{
	"Doubao-Seedance-2.0":      0.6087,
	"Doubao-Seedance-2.0-fast": 0.5946,
	"Doubao-Seedance-2.0-mini": 0.6087,
}

// GetVideoInputRatio 返回指定模型的视频输入计费倍数。
func GetVideoInputRatio(modelName string) (float64, bool) {
	r, ok := videoInputRatioMap[modelName]
	return r, ok
}
