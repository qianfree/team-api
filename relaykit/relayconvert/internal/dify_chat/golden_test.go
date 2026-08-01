package dify_chat

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

func difyGoldenMeta() *convmeta.Values {
	return &convmeta.Values{ChannelMetaAttached: true, OriginModelName: "gpt-4", UpstreamModelName: "dify-bot", IsStream: false}
}

func TestGolden_OpenAI_To_Dify_Request(t *testing.T) {
	converter := &OpenAIToDifyRequestConverter{}
	ctx := context.Background()
	info := difyGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_oai_to_dify_request.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatOpenAI || tc.To != types.RelayFormatDify {
				t.Skip("not an OpenAI→Dify request test")
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

func TestGolden_Dify_To_OpenAI_Response(t *testing.T) {
	converter := &DifyToOpenAIResponseConverter{}
	ctx := context.Background()
	info := difyGoldenMeta()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_dify_to_oai_response.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatDify || tc.To != types.RelayFormatOpenAI {
				t.Skip("not a Dify→OpenAI response test")
			}
			resp, err := kitutil.Any2Type[dto.DifyBlockingResponse](tc.Response)
			if err != nil {
				t.Fatalf("map→Dify response: %v", err)
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
			// 响应 Created 为实时时间戳，比较时忽略。
			if !goldentest.EqualExcluding(result, tc.ExpectedResponse, "created") {
				t.Errorf("result does not match golden\nGot:  %+v\nWant: %+v", result, tc.ExpectedResponse)
			}
		})
	}
}

func TestGolden_Dify_To_OpenAI_Stream(t *testing.T) {
	converter := &DifyToOpenAIStreamConverter{}
	ctx := context.Background()
	info := &convmeta.Values{ChannelMetaAttached: true, OriginModelName: "gpt-4", IsStream: true}

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("read golden dir: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_dify_to_oai_stream.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatDify || tc.To != types.RelayFormatOpenAI {
				t.Skip("not a Dify→OpenAI stream test")
			}
			var chunks []any
			chunkWriter := func(chunk any) error { chunks = append(chunks, chunk); return nil }
			if err := converter.ConvertStreamResponse(ctx, info, strings.NewReader(tc.StreamData), chunkWriter); err != nil {
				t.Fatalf("ConvertStreamResponse: %v", err)
			}
			if *goldentest.Update {
				tc.ExpectedStreamChunks = chunks
				goldentest.Save(t, filepath.Join("golden", file.Name()), tc)
				return
			}
			// chunk 的 id 为 chatcmpl-<now>（实时），忽略 id/created 保持稳定。
			if !goldentest.EqualChunksExcluding(chunks, tc.ExpectedStreamChunks, "id") {
				t.Errorf("stream chunks do not match golden\ngot=%+v\nwant=%+v", chunks, tc.ExpectedStreamChunks)
			}
		})
	}
}
