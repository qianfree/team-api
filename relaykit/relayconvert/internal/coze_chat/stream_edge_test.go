package coze_chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/stretchr/testify/assert"
)

// collectChunks 返回一个收集 *dto.ChatCompletionStreamResponse 的 chunkWriter。
func collectChunks(out *[]*dto.ChatCompletionStreamResponse) func(any) error {
	return func(chunk any) error {
		if sc, ok := chunk.(*dto.ChatCompletionStreamResponse); ok {
			*out = append(*out, sc)
		}
		return nil
	}
}

func TestCozeStream_ErrorEvent(t *testing.T) {
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: error\ndata: {\"code\":500}\n"
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(nil))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "coze stream error")
}

func TestCozeStream_ContextCanceled(t *testing.T) {
	c := &CozeToOpenAIStreamConverter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sse := "event: conversation.message.delta\ndata: {\"role\":\"assistant\",\"type\":\"answer\",\"content\":\"hi\"}\n"
	err := c.ConvertStreamResponse(ctx, nil, strings.NewReader(sse), collectChunks(nil))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCozeStream_ChunkWriterError(t *testing.T) {
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"hi"}` + "\n"
	boomer := func(any) error { return errors.New("boom") }
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), boomer)
	assert.ErrorContains(t, err, "boom")
}

func TestCozeStream_AbnormalEOF_NoAnswer(t *testing.T) {
	// 仅有非 answer 的 delta，无 done/completed：循环不产出，结尾补 role + finish
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"follow_up","content":"x"}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	// role chunk + finish chunk
	if assert.Len(t, chunks, 2) {
		assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
		assert.NotNil(t, chunks[1].Choices[0].FinishReason)
	}
}

func TestCozeStream_AbnormalEOF_WithContent(t *testing.T) {
	// 有 answer delta 但无 done/completed：结尾补 finish
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"hi"}` + "\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	// role + content + finish
	if assert.Len(t, chunks, 3) {
		assert.NotNil(t, chunks[2].Choices[0].FinishReason)
	}
}

func TestCozeStream_DoneWithoutCompleted(t *testing.T) {
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"hi"}` + "\n" +
		"event: done\ndata: {}\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	// role + content + finish(done 兜底)
	if assert.Len(t, chunks, 3) {
		assert.NotNil(t, chunks[2].Choices[0].FinishReason)
	}
}

func TestCozeStream_MalformedAndEmptyData(t *testing.T) {
	// 空 data 与畸形 JSON 应被跳过，不 panic、不产出内容 chunk
	c := &CozeToOpenAIStreamConverter{}
	sse := "event: conversation.message.delta\ndata: \n" +
		"event: conversation.message.delta\ndata: not-json\n" +
		"event: done\ndata: {}\n"
	var chunks []*dto.ChatCompletionStreamResponse
	err := c.ConvertStreamResponse(context.Background(), nil, strings.NewReader(sse), collectChunks(&chunks))
	assert.NoError(t, err)
	// 仅 done 兜底的 finish chunk（无内容）
	if assert.Len(t, chunks, 1) {
		assert.NotNil(t, chunks[0].Choices[0].FinishReason)
	}
}

func TestCozeStream_OriginModelName(t *testing.T) {
	c := &CozeToOpenAIStreamConverter{}
	info := &convmeta.Values{OriginModelName: "my-model"}
	sse := "event: conversation.message.delta\n" +
		`data: {"role":"assistant","type":"answer","content":"hi"}` + "\n" +
		"event: done\ndata: {}\n"
	var chunks []*dto.ChatCompletionStreamResponse
	_ = c.ConvertStreamResponse(context.Background(), info, strings.NewReader(sse), collectChunks(&chunks))
	if assert.NotEmpty(t, chunks) {
		assert.Equal(t, "my-model", chunks[0].Model)
	}
}
