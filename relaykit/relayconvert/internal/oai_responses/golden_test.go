package oai_responses

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

// r2cGoldenMeta 提供 r2c 请求转换所需的 Meta（映射渠道：上游模型名与客户端不同）。
func r2cGoldenMeta() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gpt-4o",
		UpstreamModelName:   "gpt-4o-2024-11-20",
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
