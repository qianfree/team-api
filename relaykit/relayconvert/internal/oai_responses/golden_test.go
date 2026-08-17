package oai_responses

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/goldentest"
	"github.com/qianfree/team-api/relaykit/relayconvert/kitutil"
	"github.com/qianfree/team-api/relaykit/types"
)

// r2cGoldenMeta 提供 r2c 请求转换所需的 Meta（映射渠道：上游模型名与客户端不同）。
func r2cGoldenMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gpt-4o",
		UpstreamModelName:   "gpt-4o-2024-11-20",
	}
}

func TestGolden_OAIChat_To_Responses_Request(t *testing.T) {
	converter := &OpenAIChatToResponsesRequestConverter{}
	ctx := context.Background()
	info := r2cGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_c2r_request.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatOpenAI || tc.To != types.RelayFormatOpenAIResponses {
				t.Skip("not an OpenAI chat→Responses request test")
			}
			req, err := kitutil.Any2Type[dto.GeneralOpenAIRequest](tc.Request)
			if err != nil {
				t.Fatalf("map→chat request: %v", err)
			}
			result, err := converter.ConvertRequest(ctx, info, &req)
			if err != nil {
				t.Fatalf("ConvertRequest: %v", err)
			}
			if *goldentest.Update {
				tc.ExpectedRequest = result
				goldentest.Save(t, filepath.Join("golden", file.Name()), tc)
				return
			}
			if !goldentest.Equal(result, tc.ExpectedRequest) {
				t.Errorf("result does not match golden\nGot:  %+v\nWant: %+v", result, tc.ExpectedRequest)
			}
		})
	}
}

func TestGolden_OAIChat_To_Responses_Response(t *testing.T) {
	converter := &OpenAIChatToResponsesResponseConverter{}
	ctx := context.Background()
	info := r2cGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_chat_to_responses_response.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatOpenAI || tc.To != types.RelayFormatOpenAIResponses {
				t.Skip("not an OpenAI chat→Responses response test")
			}
			resp, err := kitutil.Any2Type[dto.ChatCompletionResponse](tc.Response)
			if err != nil {
				t.Fatalf("map→chat response: %v", err)
			}
			result, _, err := converter.ConvertResponse(ctx, info, &resp)
			if err != nil {
				t.Fatalf("ConvertResponse: %v", err)
			}
			if *goldentest.Update {
				tc.ExpectedResponse = result
				goldentest.Save(t, filepath.Join("golden", file.Name()), tc)
				return
			}
			if !goldentest.EqualExcluding(result, tc.ExpectedResponse, "created_at", "completed_at") {
				t.Errorf("result does not match golden\nGot:  %+v\nWant: %+v", result, tc.ExpectedResponse)
			}
		})
	}
}

func TestGolden_Claude_To_Responses_Response(t *testing.T) {
	converter := &ClaudeToResponsesResponseConverter{}
	ctx := context.Background()
	info := r2cGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_claude_to_responses_response.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatClaude || tc.To != types.RelayFormatOpenAIResponses {
				t.Skip("not a Claude→Responses response test")
			}
			resp, err := kitutil.Any2Type[dto.ClaudeResponse](tc.Response)
			if err != nil {
				t.Fatalf("map→Claude response: %v", err)
			}
			result, _, err := converter.ConvertResponse(ctx, info, &resp)
			if err != nil {
				t.Fatalf("ConvertResponse: %v", err)
			}
			if *goldentest.Update {
				tc.ExpectedResponse = result
				goldentest.Save(t, filepath.Join("golden", file.Name()), tc)
				return
			}
			if !goldentest.EqualExcluding(result, tc.ExpectedResponse, "created_at", "completed_at") {
				t.Errorf("result does not match golden\nGot:  %+v\nWant: %+v", result, tc.ExpectedResponse)
			}
		})
	}
}

func TestGolden_Claude_To_Responses_Stream(t *testing.T) {
	converter := &ClaudeToResponsesStreamConverter{}
	ctx := context.Background()
	info := r2cGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_claude_to_responses_stream.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatClaude || tc.To != types.RelayFormatOpenAIResponses {
				t.Skip("not a Claude→Responses stream test")
			}
			// 固定时钟保证 resp_/msg_ ID 与 created_at 确定
			originalNow := NowFunc
			NowFunc = func() time.Time { return time.Unix(1730000000, 1000000) }
			defer func() { NowFunc = originalNow }()

			var chunks []any
			chunkWriter := func(chunk any) error {
				chunks = append(chunks, chunk)
				return nil
			}
			if err := converter.ConvertStreamResponse(ctx, info, strings.NewReader(tc.StreamData), chunkWriter); err != nil {
				t.Fatalf("ConvertStreamResponse: %v", err)
			}
			if *goldentest.Update {
				tc.ExpectedStreamChunks = chunks
				goldentest.Save(t, filepath.Join("golden", file.Name()), tc)
				return
			}
			if !goldentest.EqualChunksExcluding(chunks, tc.ExpectedStreamChunks) {
				t.Errorf("stream chunks do not match golden\nGot %d chunks, want %d", len(chunks), len(tc.ExpectedStreamChunks))
			}
		})
	}
}

func TestGolden_Responses_To_OAIChat_Request(t *testing.T) {
	converter := &ResponsesToOpenAIChatRequestConverter{}
	ctx := context.Background()
	info := r2cGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_r2c_request.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatOpenAIResponses || tc.To != types.RelayFormatOpenAI {
				t.Skip("not a Responses→OpenAI chat request test")
			}
			req, err := kitutil.Any2Type[dto.OpenAIResponsesRequest](tc.Request)
			if err != nil {
				t.Fatalf("map→Responses request: %v", err)
			}
			result, err := converter.ConvertRequest(ctx, info, &req)
			if err != nil {
				t.Fatalf("ConvertRequest: %v", err)
			}
			if *goldentest.Update {
				tc.ExpectedRequest = result
				goldentest.Save(t, filepath.Join("golden", file.Name()), tc)
				return
			}
			if !goldentest.Equal(result, tc.ExpectedRequest) {
				t.Errorf("result does not match golden\nGot:  %+v\nWant: %+v", result, tc.ExpectedRequest)
			}
		})
	}
}
