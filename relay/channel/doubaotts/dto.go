package doubaotts

// DoubaoTTSRequest is the request body for /api/v3/tts/unidirectional
type DoubaoTTSRequest struct {
	ReqParams ReqParams `json:"req_params"`
}

// ReqParams is the inner parameters of the request
type ReqParams struct {
	Text        string      `json:"text"`
	Speaker     string      `json:"speaker,omitempty"`
	AudioParams AudioParams `json:"audio_params,omitempty"`
}

// AudioParams is the audio output configuration
type AudioParams struct {
	Format     string `json:"format,omitempty"`
	SampleRate int    `json:"sample_rate,omitempty"`
}

// DoubaoTTSResponse is a single chunk in the streaming response.
// The /api/v3/tts/unidirectional endpoint returns multiple JSON objects
// (one per audio chunk) separated by newlines.
type DoubaoTTSResponse struct {
	Code     int      `json:"code"`
	Message  string   `json:"message"`
	Data     string   `json:"data"`
	Sentence Sentence `json:"sentence,omitempty"`
	Usage    Usage    `json:"usage,omitempty"`
}

// Sentence contains word-level timing information
type Sentence struct {
	Phonemes []any  `json:"phonemes,omitempty"`
	Text     string `json:"text,omitempty"`
	Words    []Word `json:"words,omitempty"`
}

// Word contains timing for a single word
type Word struct {
	Confidence float64 `json:"confidence,omitempty"`
	EndTime    float64 `json:"endTime,omitempty"`
	StartTime  float64 `json:"startTime,omitempty"`
	Word       string  `json:"word,omitempty"`
}

// Usage contains billing information
type Usage struct {
	TextWords int `json:"text_words,omitempty"`
}
