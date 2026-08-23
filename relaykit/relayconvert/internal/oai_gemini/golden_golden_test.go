package oai_gemini

import (
	"context"
	"encoding/json"
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

func TestGolden_OpenAI_To_Gemini_Request(t *testing.T) {
	converter := &OpenAIToGeminiRequestConverter{}
	ctx := context.Background()

	testDir := "golden"
	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_oai_to_gemini_request.json") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			testCase := goldentest.Load(t, filepath.Join(testDir, file.Name()))

			if testCase.From != types.RelayFormatOpenAI || testCase.To != types.RelayFormatGemini {
				t.Skip("Not an OpenAI to Gemini request test")
			}

			openaiReq, err := mapToOpenAIRequest(testCase.Request)
			if err != nil {
				t.Fatalf("Failed to convert request to OpenAI format: %v", err)
			}

			result, err := converter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			if *goldentest.Update {
				testCase.ExpectedRequest = result
				goldentest.Save(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			if !goldentest.Equal(result, testCase.ExpectedRequest) {
				t.Errorf("Result does not match expected output")
				t.Logf("Got:      %+v", result)
				t.Logf("Expected: %+v", testCase.ExpectedRequest)
			}
		})
	}
}

func TestGolden_Gemini_To_OpenAI_Response(t *testing.T) {
	converter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	// 冻结时钟：响应 ID 兜底与 created 均由 NowFunc 派生，保证 golden 确定
	originalNow := NowFunc
	NowFunc = func() time.Time { return time.Unix(1700000000, 0) }
	defer func() { NowFunc = originalNow }()

	testDir := "golden"
	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_gemini_to_oai_response.json") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			testCase := goldentest.Load(t, filepath.Join(testDir, file.Name()))

			if testCase.From != types.RelayFormatGemini || testCase.To != types.RelayFormatOpenAI {
				t.Skip("Not a Gemini to OpenAI response test")
			}

			geminiResp, err := mapToGeminiResponse(testCase.Response)
			if err != nil {
				t.Fatalf("Failed to convert response to Gemini format: %v", err)
			}

			result, err := converter.ConvertResponse(ctx, nil, geminiResp)
			if err != nil {
				t.Fatalf("ConvertResponse failed: %v", err)
			}

			if *goldentest.Update {
				testCase.ExpectedResponse = result
				goldentest.Save(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			if !goldentest.Equal(result, testCase.ExpectedResponse) {
				t.Errorf("Result does not match expected output")
				t.Logf("Got:      %+v", result)
				t.Logf("Expected: %+v", testCase.ExpectedResponse)
			}
		})
	}
}

func TestGolden_Gemini_To_OpenAI_Stream(t *testing.T) {
	converter := &GeminiToOpenAIStreamConverter{}
	ctx := context.Background()

	// 冻结时钟：响应 ID 兜底与 created 均由 NowFunc 派生，保证 golden 确定
	originalNow := NowFunc
	NowFunc = func() time.Time { return time.Unix(1700000000, 0) }
	defer func() { NowFunc = originalNow }()

	testDir := "golden"
	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_gemini_to_oai_stream.json") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			testCase := goldentest.Load(t, filepath.Join(testDir, file.Name()))

			if testCase.From != types.RelayFormatGemini || testCase.To != types.RelayFormatOpenAI {
				t.Skip("Not a Gemini to OpenAI stream test")
			}

			var chunks []any
			chunkWriter := func(chunk any) error {
				chunks = append(chunks, chunk)
				return nil
			}

			reader := strings.NewReader(testCase.StreamData)
			if err := converter.ConvertStreamResponse(ctx, nil, reader, chunkWriter); err != nil {
				t.Fatalf("ConvertStreamResponse failed: %v", err)
			}

			if *goldentest.Update {
				testCase.ExpectedStreamChunks = chunks
				goldentest.Save(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			if len(chunks) != len(testCase.ExpectedStreamChunks) {
				t.Errorf("Chunk count mismatch: got %d, want %d", len(chunks), len(testCase.ExpectedStreamChunks))
				return
			}

			for i, chunk := range chunks {
				if !goldentest.Equal(chunk, testCase.ExpectedStreamChunks[i]) {
					t.Errorf("Chunk %d does not match expected output", i)
					t.Logf("Got:      %+v", chunk)
					t.Logf("Expected: %+v", testCase.ExpectedStreamChunks[i])
				}
			}
		})
	}
}

func TestGolden_Roundtrip_OpenAI_Gemini(t *testing.T) {
	reqConverter := &OpenAIToGeminiRequestConverter{}
	respConverter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

	// 冻结时钟：响应 ID 兜底与 created 均由 NowFunc 派生，保证 golden 确定
	originalNow := NowFunc
	NowFunc = func() time.Time { return time.Unix(1700000000, 0) }
	defer func() { NowFunc = originalNow }()

	testDir := "golden"
	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_roundtrip.json") {
			continue
		}

		t.Run(file.Name(), func(t *testing.T) {
			testCase := goldentest.Load(t, filepath.Join(testDir, file.Name()))

			openaiReq, err := mapToOpenAIRequest(testCase.Request)
			if err != nil {
				t.Fatalf("Failed to convert request to OpenAI format: %v", err)
			}

			geminiResp, err := mapToGeminiResponse(testCase.Response)
			if err != nil {
				t.Fatalf("Failed to convert response to Gemini format: %v", err)
			}

			geminiReq, err := reqConverter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("OpenAI -> Gemini request conversion failed: %v", err)
			}

			openaiResp, err := respConverter.ConvertResponse(ctx, nil, geminiResp)
			if err != nil {
				t.Fatalf("Gemini -> OpenAI response conversion failed: %v", err)
			}

			if *goldentest.Update {
				testCase.ExpectedRequest = geminiReq
				testCase.ExpectedResponse = openaiResp
				goldentest.Save(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			if !goldentest.Equal(geminiReq, testCase.ExpectedRequest) {
				t.Errorf("Gemini request does not match expected")
			}

			if !goldentest.Equal(openaiResp, testCase.ExpectedResponse) {
				t.Errorf("OpenAI response does not match expected")
			}
		})
	}
}

// mapToOpenAIRequest 将 map 转换为 *dto.GeneralOpenAIRequest
func mapToOpenAIRequest(m any) (*dto.GeneralOpenAIRequest, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var req dto.GeneralOpenAIRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// mapToGeminiResponse 将 map 转换为 *dto.GeminiChatResponse
func mapToGeminiResponse(m any) (*dto.GeminiChatResponse, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var resp dto.GeminiChatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func TestGolden_Gemini_To_OAIChat_Request(t *testing.T) {
	converter := &GeminiToOpenAIRequestConverter{}
	ctx := context.Background()
	info := convmetaValuesForG2O()

	files, err := os.ReadDir("golden")
	if err != nil {
		t.Fatalf("Failed to read golden directory: %v", err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "_g2o_request.json") {
			continue
		}
		t.Run(file.Name(), func(t *testing.T) {
			tc := goldentest.Load(t, filepath.Join("golden", file.Name()))
			if tc.From != types.RelayFormatGemini || tc.To != types.RelayFormatOpenAI {
				t.Skip("not a Gemini→OpenAI chat request test")
			}
			req, err := kitutil.Any2Type[dto.GeminiChatRequest](tc.Request)
			if err != nil {
				t.Fatalf("map→Gemini request: %v", err)
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

// convmetaValuesForG2O g2o golden 测试用 Meta（映射渠道）。
func convmetaValuesForG2O() *convmeta.Values {
	return &convmeta.Values{
		ChannelMetaAttached: true,
		OriginModelName:     "gpt-4o",
		UpstreamModelName:   "gpt-4o-2024-11-20",
	}
}
