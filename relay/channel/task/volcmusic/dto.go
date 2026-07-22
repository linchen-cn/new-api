package volcmusic

// GenSongRequest 生成人声歌曲（后付费）的请求体，火山引擎使用 PascalCase。
// 可选的标量字段使用指针类型 + omitempty，以区分「未设置」与「显式零值」。
type GenSongRequest struct {
	Prompt       string `json:"Prompt,omitempty"`
	Lyrics       string `json:"Lyrics,omitempty"`
	ModelVersion string `json:"ModelVersion,omitempty"`
	Genre        string `json:"Genre,omitempty"`
	Mood         string `json:"Mood,omitempty"`
	Gender       string `json:"Gender,omitempty"`
	Timbre       string `json:"Timbre,omitempty"`
	Duration     *int   `json:"Duration,omitempty"`
	TosBucket    string `json:"TosBucket,omitempty"`
	CallbackURL  string `json:"CallbackURL,omitempty"`
	Key          string `json:"Key,omitempty"`
	Kmode        *int   `json:"Kmode,omitempty"`
	Tempo        *int   `json:"Tempo,omitempty"`
	Instrument   *bool  `json:"Instrument,omitempty"`
	Scene        string `json:"Scene,omitempty"`
	Lang         string `json:"Lang,omitempty"`
}

// GenBGMRequest 生成纯音乐 BGM（后付费）的请求体。Text 为必选字段。
type GenBGMRequest struct {
	Text               string `json:"Text"`
	Duration           *int   `json:"Duration,omitempty"`
	TosBucket          string `json:"TosBucket,omitempty"`
	CallbackURL        string `json:"CallbackURL,omitempty"`
	EnableInputRewrite *bool  `json:"EnableInputRewrite,omitempty"`
	Segments           any    `json:"Segments,omitempty"`
	Version            string `json:"Version,omitempty"`
	ImplicitWaterMark  *bool  `json:"ImplicitWaterMark,omitempty"`
}

// QueryTaskRequest 查询任务的请求体。
type QueryTaskRequest struct {
	TaskID string `json:"TaskID"`
}

// SubmitResponse 提交任务（GenSongForTime / GenBGMForTime）的响应。
type SubmitResponse struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
	Result  struct {
		TaskID            string `json:"TaskID"`
		PredictedWaitTime int    `json:"PredictedWaitTime"`
	} `json:"Result"`
}

// SongDetail 查询任务成功后返回的歌曲详情。
type SongDetail struct {
	AudioUrl string  `json:"AudioUrl"`
	Captions string  `json:"Captions"`
	Lyrics   string  `json:"Lyrics"`
	Duration float64 `json:"Duration"`
	Genre    string  `json:"Genre"`
	Mood     string  `json:"Mood"`
	Gender   string  `json:"Gender"`
}

// FailureReason 任务失败原因，任务执行成功时为 null。
type FailureReason struct {
	Code int    `json:"Code"`
	Msg  string `json:"Msg"`
}

// QueryResponse 查询任务（QuerySong）的响应。
// Status: 0->等待中, 1->处理中, 2->成功, 3->失败
type QueryResponse struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
	Result  struct {
		TaskID        string         `json:"TaskID"`
		Status        int            `json:"Status"`
		Progress      int            `json:"Progress"`
		FailureReason *FailureReason `json:"FailureReason"`
		SongDetail    *SongDetail    `json:"SongDetail"`
	} `json:"Result"`
}
