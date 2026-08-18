package oai_chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/goldentest"
	"github.com/qianfree/team-api/relaykit/relayconvert/kitutil"
	"github.com/qianfree/team-api/relaykit/types"
)

// claudeGoldenMeta 提供 Claude 请求转换所需的 Meta（max_tokens 默认值 + 模型名）。
func claudeGoldenMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gpt-4",
		UpstreamModelName:   "claude-3-5-sonnet-20241022",
		Options: &convmeta.Options{
			Claude: convmeta.ClaudeOptions{DefaultMaxTokens: func(string) int { return 4096 }},
		},
	}
}

func TestGolden_OpenAI_To_Claude_Request(t *testing.T) {
	converter := &OpenAIToClaudeRequestConverter{}
	ctx := context.Background()
	info := claudeGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_oai_to_claude_request.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatOpenAI || tc.To != types.RelayFormatClaude {
				t.Skip("not an OpenAI→Claude request test")
			}
			req, err := kitutil.Any2Type[dto.GeneralOpenAIRequest](tc.Request)
			if err != nil {
				t.Fatalf("map→OpenAI request: %v", err)
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

func TestGolden_Claude_To_OpenAI_Response(t *testing.T) {
	converter := &ClaudeToOpenAIResponseConverter{}
	ctx := context.Background()
	info := claudeGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_claude_to_oai_response.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatClaude || tc.To != types.RelayFormatOpenAI {
				t.Skip("not a Claude→OpenAI response test")
			}
			resp, err := kitutil.Any2Type[dto.ClaudeResponse](tc.Response)
			if err != nil {
				t.Fatalf("map→Claude response: %v", err)
			}
			result, err := converter.ConvertResponse(ctx, info, &resp)
			if err != nil {
				t.Fatalf("ConvertResponse: %v", err)
			}
			if *goldentest.Update {
				tc.ExpectedResponse = result
				goldentest.Save(t, filepath.Join("golden", file.Name()), tc)
				return
			}
			// 响应 Created 为 time.Now().Unix()（非确定），比较时忽略该字段以保持 golden 稳定。
			if !goldentest.EqualExcluding(result, tc.ExpectedResponse, "created") {
				t.Errorf("result does not match golden\nGot:  %+v\nWant: %+v", result, tc.ExpectedResponse)
			}
		})
	}
}

func TestGolden_Claude_To_OAIChat_Request(t *testing.T) {
	converter := &ClaudeToOpenAIRequestConverter{}
	ctx := context.Background()
	info := &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gpt-4o",
		UpstreamModelName:   "gpt-4o-2024-11-20",
	}

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_c2o_request.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatClaude || tc.To != types.RelayFormatOpenAI {
				t.Skip("not a Claude→OpenAI chat request test")
			}
			req, err := kitutil.Any2Type[dto.ClaudeRequest](tc.Request)
			if err != nil {
				t.Fatalf("map→Claude request: %v", err)
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
