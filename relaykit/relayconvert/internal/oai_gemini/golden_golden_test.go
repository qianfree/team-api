package oai_gemini

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/types"
)

var updateGolden = flag.Bool("update-golden", false, "Update golden test files")

// GoldenTestCase represents a complete conversion test case
type GoldenTestCase struct {
	Name     string        `json:"name"`
	From     types.RelayFormat `json:"from"`
	To       types.RelayFormat `json:"to"`
	Request  any           `json:"request,omitempty"`
	Response any           `json:"response,omitempty"`
	StreamData string       `json:"stream_data,omitempty"`

	// Expected outputs
	ExpectedRequest  any     `json:"expected_request,omitempty"`
	ExpectedResponse any     `json:"expected_response,omitempty"`
	ExpectedStreamChunks []any `json:"expected_stream_chunks,omitempty"`

	// Conversion metadata
	ConverterID string `json:"converter_id"`
}

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
			testCase := loadGoldenTestCase(t, filepath.Join(testDir, file.Name()))

			if testCase.From != types.RelayFormatOpenAI || testCase.To != types.RelayFormatGemini {
				t.Skip("Not an OpenAI to Gemini request test")
			}

			// Convert request map to *dto.GeneralOpenAIRequest
			openaiReq, err := mapToOpenAIRequest(testCase.Request)
			if err != nil {
				t.Fatalf("Failed to convert request to OpenAI format: %v", err)
			}

			result, err := converter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("ConvertRequest failed: %v", err)
			}

			if *updateGolden {
				testCase.ExpectedRequest = result
				saveGoldenTestCase(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			if !compareJSON(result, testCase.ExpectedRequest) {
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
			testCase := loadGoldenTestCase(t, filepath.Join(testDir, file.Name()))

			if testCase.From != types.RelayFormatGemini || testCase.To != types.RelayFormatOpenAI {
				t.Skip("Not a Gemini to OpenAI response test")
			}

			// Convert response map to *dto.GeminiChatResponse
			geminiResp, err := mapToGeminiResponse(testCase.Response)
			if err != nil {
				t.Fatalf("Failed to convert response to Gemini format: %v", err)
			}

			result, err := converter.ConvertResponse(ctx, nil, geminiResp)
			if err != nil {
				t.Fatalf("ConvertResponse failed: %v", err)
			}

			if *updateGolden {
				testCase.ExpectedResponse = result
				saveGoldenTestCase(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			if !compareJSON(result, testCase.ExpectedResponse) {
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
			testCase := loadGoldenTestCase(t, filepath.Join(testDir, file.Name()))

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

			if *updateGolden {
				testCase.ExpectedStreamChunks = chunks
				saveGoldenTestCase(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			if len(chunks) != len(testCase.ExpectedStreamChunks) {
				t.Errorf("Chunk count mismatch: got %d, want %d", len(chunks), len(testCase.ExpectedStreamChunks))
				return
			}

			for i, chunk := range chunks {
				if !compareJSON(chunk, testCase.ExpectedStreamChunks[i]) {
					t.Errorf("Chunk %d does not match expected output", i)
					t.Logf("Got:      %+v", chunk)
					t.Logf("Expected: %+v", testCase.ExpectedStreamChunks[i])
				}
			}
		})
	}
}

func TestGolden_Roundtrip_OpenAI_Gemini(t *testing.T) {
	// Test roundtrip conversion: OpenAI -> Gemini -> OpenAI
	reqConverter := &OpenAIToGeminiRequestConverter{}
	respConverter := &GeminiToOpenAIResponseConverter{}
	ctx := context.Background()

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
			testCase := loadGoldenTestCase(t, filepath.Join(testDir, file.Name()))

			// Convert maps to proper types
			openaiReq, err := mapToOpenAIRequest(testCase.Request)
			if err != nil {
				t.Fatalf("Failed to convert request to OpenAI format: %v", err)
			}

			geminiResp, err := mapToGeminiResponse(testCase.Response)
			if err != nil {
				t.Fatalf("Failed to convert response to Gemini format: %v", err)
			}

			// OpenAI request -> Gemini request
			geminiReq, err := reqConverter.ConvertRequest(ctx, nil, openaiReq)
			if err != nil {
				t.Fatalf("OpenAI -> Gemini request conversion failed: %v", err)
			}

			// Gemini response -> OpenAI response (simulating response)
			openaiResp, err := respConverter.ConvertResponse(ctx, nil, geminiResp)
			if err != nil {
				t.Fatalf("Gemini -> OpenAI response conversion failed: %v", err)
			}

			if *updateGolden {
				testCase.ExpectedRequest = geminiReq
				testCase.ExpectedResponse = openaiResp
				saveGoldenTestCase(t, filepath.Join(testDir, file.Name()), testCase)
				return
			}

			// Verify the roundtrip preserves critical information
			if !compareJSON(geminiReq, testCase.ExpectedRequest) {
				t.Errorf("Gemini request does not match expected")
			}

			if !compareJSON(openaiResp, testCase.ExpectedResponse) {
				t.Errorf("OpenAI response does not match expected")
			}
		})
	}
}

// Helper functions

func loadGoldenTestCase(t *testing.T, path string) GoldenTestCase {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read golden file: %v", err)
	}

	var testCase GoldenTestCase
	if err := json.Unmarshal(data, &testCase); err != nil {
		t.Fatalf("Failed to parse golden file: %v", err)
	}

	return testCase
}

func saveGoldenTestCase(t *testing.T, path string, testCase GoldenTestCase) {
	data, err := json.MarshalIndent(testCase, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal golden file: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write golden file: %v", err)
	}

	t.Logf("Updated golden file: %s", path)
}

func compareJSON(a, b any) bool {
	// Marshal both to JSON
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}

	// Unmarshal both to maps for consistent comparison
	var aMap, bMap map[string]any
	if err := json.Unmarshal(aJSON, &aMap); err != nil {
		return false
	}
	if err := json.Unmarshal(bJSON, &bMap); err != nil {
		return false
	}

	// Compare the maps
	return compareMaps(aMap, bMap)
}

// compareMaps recursively compares two maps
func compareMaps(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}

	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}

		if !compareValues(av, bv) {
			return false
		}
	}

	return true
}

// compareValues compares two values (including nested structures)
func compareValues(a, b any) bool {
	// Handle nil values
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle maps
	if am, ok := a.(map[string]any); ok {
		if bm, ok := b.(map[string]any); ok {
			return compareMaps(am, bm)
		}
		return false
	}

	// Handle slices
	if as, ok := a.([]any); ok {
		if bs, ok := b.([]any); ok {
			if len(as) != len(bs) {
				return false
			}
			for i := range as {
				if !compareValues(as[i], bs[i]) {
					return false
				}
			}
			return true
		}
		return false
	}

	// Handle primitive types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// mapToOpenAIRequest converts a map to *dto.GeneralOpenAIRequest
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

// mapToGeminiResponse converts a map to *dto.GeminiChatResponse
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
