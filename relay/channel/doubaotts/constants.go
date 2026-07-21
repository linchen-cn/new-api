package doubaotts

var ModelList = []string{
	"seed-tts-2.0",
}

var ChannelName = "doubaotts"

// resourceIDMap maps model names to their X-Api-Resource-Id values.
var resourceIDMap = map[string]string{
	"seed-tts-2.0": "seed-tts-2.0",
}

func GetResourceID(modelName string) string {
	if rid, ok := resourceIDMap[modelName]; ok {
		return rid
	}
	return modelName
}
