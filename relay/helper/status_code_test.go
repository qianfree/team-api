package helper

import (
	"encoding/json"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
)

func TestRemapStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		mapping  string
		wantCode int
		wantNil  bool
	}{
		{
			name:    "nil error returns nil",
			err:     nil,
			mapping: `{"429": 500}`,
			wantNil: true,
		},
		{
			name:     "empty mapping returns same error",
			err:      constant.NewUpstreamError(429, "rate limited", nil),
			mapping:  "",
			wantCode: 429,
		},
		{
			name:     "remap 429 to 500",
			err:      constant.NewUpstreamError(429, "rate limited", nil),
			mapping:  `{"429": 500}`,
			wantCode: 500,
		},
		{
			name:     "remap 403 to 500",
			err:      constant.NewUpstreamError(403, "forbidden", nil),
			mapping:  `{"429": 500, "403": 500}`,
			wantCode: 500,
		},
		{
			name:     "no matching mapping keeps original",
			err:      constant.NewUpstreamError(400, "bad request", nil),
			mapping:  `{"429": 500, "403": 500}`,
			wantCode: 400,
		},
		{
			name:     "invalid JSON mapping returns original RelayError",
			err:      constant.NewUpstreamError(429, "rate limited", nil),
			mapping:  `{invalid}`,
			wantCode: 429,
		},
		{
			name:     "non-RelayError returns same error unchanged",
			err:      json.Unmarshal([]byte("{bad}"), &map[string]any{}),
			mapping:  `{"400": 500}`,
			wantCode: 0, // not a RelayError, won't be modified
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemapStatusCode(tt.err, tt.mapping)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			relayErr, ok := result.(*constant.RelayError)
			if tt.wantCode == 0 {
				// Not a RelayError — just check it's not nil and not a RelayError
				if ok {
					t.Errorf("expected non-RelayError, got RelayError with code %d", relayErr.StatusCode)
				}
				return
			}

			if !ok {
				t.Fatalf("expected *RelayError, got %T", result)
			}
			if relayErr.StatusCode != tt.wantCode {
				t.Errorf("StatusCode = %d, want %d", relayErr.StatusCode, tt.wantCode)
			}
		})
	}
}
