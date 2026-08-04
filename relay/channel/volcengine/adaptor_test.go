package volcengine

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripDataURLPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		stripped bool
	}{
		{
			name:     "data URL with mp3",
			input:    "data:audio/mpeg;base64,SUQzBAAAAAAA",
			want:     "SUQzBAAAAAAA",
			stripped: true,
		},
		{
			name:     "data URL with wav",
			input:    "data:audio/wav;base64,UklGRiQ=",
			want:     "UklGRiQ=",
			stripped: true,
		},
		{
			name:     "raw base64 without prefix",
			input:    "SUQzBAAAAAAA",
			want:     "SUQzBAAAAAAA",
			stripped: false,
		},
		{
			name:     "empty string",
			input:    "",
			want:     "",
			stripped: false,
		},
		{
			name:     "data URL without comma",
			input:    "data:audio/mpeg;base64",
			want:     "data:audio/mpeg;base64",
			stripped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, stripped := stripDataURLPrefix(tt.input)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.stripped, stripped)
		})
	}
}

func TestConvertOpenAIRequestStripsDataURLFromInputAudio(t *testing.T) {
	requestJSON := `{
		"model": "Doubao-Seed-2.0-mini",
		"messages": [
			{
				"role": "user",
				"content": [
					{
						"type": "input_audio",
						"input_audio": {
							"data": "data:audio/mpeg;base64,SUQzBAAAAAAA",
							"format": "mp3"
						}
					},
					{
						"type": "text",
						"text": "transcribe this audio"
					}
				]
			}
		]
	}`

	var request dto.GeneralOpenAIRequest
	err := common.Unmarshal([]byte(requestJSON), &request)
	require.NoError(t, err)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "Doubao-Seed-2.0-mini",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, &request)
	require.NoError(t, err)

	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)

	require.Len(t, convertedRequest.Messages, 1)
	parts := convertedRequest.Messages[0].ParseContent()
	require.Len(t, parts, 2)

	// First part should be input_audio with raw base64 (no data URL prefix)
	assert.Equal(t, dto.ContentTypeInputAudio, parts[0].Type)
	audio := parts[0].GetInputAudio()
	require.NotNil(t, audio)
	assert.Equal(t, "SUQzBAAAAAAA", audio.Data)
	assert.Equal(t, "mp3", audio.Format)
	assert.NotContains(t, audio.Data, "data:")

	// Second part should be text, unchanged
	assert.Equal(t, dto.ContentTypeText, parts[1].Type)
	assert.Equal(t, "transcribe this audio", parts[1].Text)
}

func TestConvertOpenAIRequestKeepsRawBase64InInputAudio(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "Doubao-Seed-2.0-mini",
		Messages: []dto.Message{
			{Role: "user"},
		},
	}
	request.Messages[0].SetMediaContent([]dto.MediaContent{
		{
			Type: dto.ContentTypeInputAudio,
			InputAudio: &dto.MessageInputAudio{
				Data:   "SUQzBAAAAAAA",
				Format: "mp3",
			},
		},
	})

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "Doubao-Seed-2.0-mini",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
	require.NoError(t, err)

	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)

	parts := convertedRequest.Messages[0].ParseContent()
	require.Len(t, parts, 1)
	audio := parts[0].GetInputAudio()
	require.NotNil(t, audio)
	assert.Equal(t, "SUQzBAAAAAAA", audio.Data)
}
