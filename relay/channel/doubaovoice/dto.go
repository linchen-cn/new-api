package doubaovoice

type DoubaoVoiceRequest struct {
	Model         string        `json:"model"`
	TextPrompt    string        `json:"text_prompt"`
	References    []Reference   `json:"references,omitempty"`
	Speaker       string        `json:"speaker,omitempty"`
	AudioData     string        `json:"audio_data,omitempty"`
	AudioUrl      string        `json:"audio_url,omitempty"`
	ImageData     string        `json:"image_data,omitempty"`
	ImageUrl      string        `json:"image_url,omitempty"`
	AudioConfig   AudioConfig   `json:"audio_config,omitempty"`
	Watermark     Watermark     `json:"watermark,omitempty"`
}

type Reference struct {
	Speaker   string `json:"speaker,omitempty"`
	AudioData string `json:"audio_data,omitempty"`
	AudioUrl  string `json:"audio_url,omitempty"`
	ImageData string `json:"image_data,omitempty"`
	ImageUrl  string `json:"image_url,omitempty"`
}

type AudioConfig struct {
	Format         string `json:"format,omitempty"`
	SampleRate     int    `json:"sample_rate,omitempty"`
	SpeechRate     int    `json:"speech_rate,omitempty"`
	LoudnessRate   int    `json:"loudness_rate,omitempty"`
	PitchRate      int    `json:"pitch_rate,omitempty"`
	EnableSubtitle bool   `json:"enable_subtitle,omitempty"`
}

type Watermark struct {
	AigcWatermark bool           `json:"aigc_watermark,omitempty"`
	AigcMetadata  AigcMetadata   `json:"aigc_metadata,omitempty"`
}

type AigcMetadata struct {
	Enable           bool   `json:"enable,omitempty"`
	ContentProducer  string `json:"content_producer,omitempty"`
	ProduceID        string `json:"produce_id,omitempty"`
	ContentPropagator string `json:"content_propagator,omitempty"`
	PropagateID      string `json:"propagate_id,omitempty"`
}

type DoubaoVoiceResponse struct {
	Code           int            `json:"code"`
	Message        string         `json:"message"`
	Audio          string         `json:"audio"`
	Duration       float64        `json:"duration"`
	OriginalDuration float64      `json:"original_duration"`
	URL            string         `json:"url"`
	Subtitle       Subtitle       `json:"subtitle,omitempty"`
}

type Subtitle struct {
	Text     string     `json:"text"`
	Sentences []Sentence `json:"sentences"`
}

type Sentence struct {
	StartTime int    `json:"start_time"`
	EndTime   int    `json:"end_time"`
	Text      string `json:"text"`
	Words     []Word `json:"words"`
}

type Word struct {
	StartTime int    `json:"start_time"`
	EndTime   int    `json:"end_time"`
	Text      string `json:"text"`
}
