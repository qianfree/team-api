package dify_chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/assert"
)

func collectChunks(out *[]*dto.ChatCompletionStreamResponse) func(any) error {
	return func(chunk any) error {
		if sc, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			*out = append(*out, sc)
		}
		return nil
	}
}

func TestDifyStream_ErrorEvent(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	sse := "data: {\"event\":\"error\"}\n"
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(nil))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dify stream error")
}

func TestDifyStream_ContextCanceled(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sse := "data: {\"event\":\"message\",\"answer\":\"hi\"}\n"
	err := c.ConvertStreamResponse(ctx, nil, strings.NewReader(sse), collectChunks(nil))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestDifyStream_ChunkWriterError(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	sse := "data: {\"event\":\"message\",\"answer\":\"hi\"}\n"
	boomer := func(any) error { return errors.New("boom") }
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), boomer)
	assert.ErrorContains(t, err, "boom")
}

func TestDifyStream_MessageEndWithUsage(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	sse := "data: {\"event\":\"message\",\"answer\":\"hi\"}\n" +
		`data: {"event":"message_end","metadata":{"usage":{"total_tokens":8,"prompt_tokens":5,"completion_tokens":3}}}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	// role + content + finish(带 usage)
	if assert.Len(t, chunks, 3) {
		assert.NotNil(t, chunks[2].Usage)
		assert.Equal(t, 8, chunks[2].Usage.TotalTokens)
	}
}

func TestDifyStream_MessageEndWithoutUsage(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	sse := "data: {\"event\":\"message\",\"answer\":\"hi\"}\n" +
		`data: {"event":"message_end","metadata":{"usage":{}}}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	if assert.Len(t, chunks, 3) {
		assert.Nil(t, chunks[2].Usage) // 全零 usage 不附带
	}
}

func TestDifyStream_AbnormalEOF_NoMessage(t *testing.T) {
	// 全程无 message：结尾补 role + finish
	c := &DifyToOpenAIStreamConverter{}
	sse := "data: {\"event\":\"ping\"}\n" // 未知事件，被忽略
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	if assert.Len(t, chunks, 2) {
		assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
		assert.NotNil(t, chunks[1].Choices[0].FinishReason)
	}
}

func TestDifyStream_DoneMarkerAndEmptySkipped(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	// 空行 / [DONE] / 空 data 应被跳过；仅 message_end 收尾
	sse := "\ndata: [DONE]\ndata: \n" +
		`data: {"event":"message","answer":"hi"}` + "\n" +
		`data: {"event":"message_end","metadata":{"usage":{"total_tokens":2}}}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	assert.Len(t, chunks, 3) // role + content + finish
}

func TestDifyStream_OriginModelName(t *testing.T) {
	c := &DifyToOpenAIStreamConverter{}
	info := &convmeta.Values{OriginModelName: "my-model", IsStream: true}
	sse := "data: {\"event\":\"message\",\"answer\":\"hi\"}\n" +
		`data: {"event":"message_end","metadata":{"usage":{}}}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	_ = c.ConvertStreamResponse(context.Background(), info, strings.NewReader(sse), collectChunks(&chunks))
	if assert.NotEmpty(t, chunks) {
		assert.Equal(t, "my-model", chunks[0].Model)
	}
}
