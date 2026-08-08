package relay

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/middleware"
	relay_common "github.com/qianfree/team-api/relay/common"
)

type taskModelAccessCheckerStub struct {
	relay_common.DataProvider

	memberAllowed bool
	apiKeyAllowed bool
	memberErr     error
	apiKeyErr     error
	apiKeyChecks  int
	tenantChecks  int
}

func (s *taskModelAccessCheckerStub) CheckMemberModelAccess(
	context.Context,
	int64,
	int64,
	string,
) (bool, error) {
	return s.memberAllowed, s.memberErr
}

func (s *taskModelAccessCheckerStub) CheckApiKeyModelAccess(
	context.Context,
	int64,
	string,
) (bool, error) {
	s.apiKeyChecks++
	return s.apiKeyAllowed, s.apiKeyErr
}

func (s *taskModelAccessCheckerStub) CheckTenantModelAccess(
	context.Context,
	int64,
	string,
) (bool, []int64, error) {
	s.tenantChecks++
	return false, nil, errors.New("tenant model check should not be reached")
}

func TestCheckTaskModelAccess(t *testing.T) {
	errMemberCheck := errors.New("member check failed")
	errApiKeyCheck := errors.New("api key check failed")

	tests := []struct {
		name             string
		checker          *taskModelAccessCheckerStub
		wantErr          error
		wantApiKeyChecks int
	}{
		{
			name: "allows model when both scopes allow it",
			checker: &taskModelAccessCheckerStub{
				memberAllowed: true,
				apiKeyAllowed: true,
			},
			wantApiKeyChecks: 1,
		},
		{
			name: "rejects member before checking API key",
			checker: &taskModelAccessCheckerStub{
				memberAllowed: false,
				apiKeyAllowed: true,
			},
			wantErr: relay_common.ErrMemberModelNotAllowed,
		},
		{
			name: "rejects model outside API key scope",
			checker: &taskModelAccessCheckerStub{
				memberAllowed: true,
				apiKeyAllowed: false,
			},
			wantErr:          relay_common.ErrApiKeyModelNotAllowed,
			wantApiKeyChecks: 1,
		},
		{
			name: "propagates member scope lookup error",
			checker: &taskModelAccessCheckerStub{
				memberErr: errMemberCheck,
			},
			wantErr: errMemberCheck,
		},
		{
			name: "propagates API key scope lookup error",
			checker: &taskModelAccessCheckerStub{
				memberAllowed: true,
				apiKeyErr:     errApiKeyCheck,
			},
			wantErr:          errApiKeyCheck,
			wantApiKeyChecks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkTaskModelAccess(
				context.Background(),
				tt.checker,
				1,
				2,
				3,
				"video-model",
			)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("checkTaskModelAccess() error = %v, want %v", err, tt.wantErr)
			}
			if tt.checker.apiKeyChecks != tt.wantApiKeyChecks {
				t.Fatalf("API key checks = %d, want %d", tt.checker.apiKeyChecks, tt.wantApiKeyChecks)
			}
		})
	}
}

func TestSelectTaskChannelRejectsModelOutsideApiKeyScope(t *testing.T) {
	checker := &taskModelAccessCheckerStub{
		memberAllowed: true,
		apiKeyAllowed: false,
	}
	originalProvider := relayDataProvider
	relayDataProvider = checker
	t.Cleanup(func() {
		relayDataProvider = originalProvider
	})

	r := &ghttp.Request{
		Request: httptest.NewRequest("POST", "/v1/video/generations", nil),
	}
	r.SetCtxVar(middleware.CtxKeyTenantID, int64(1))
	r.SetCtxVar(middleware.CtxKeyUserID, int64(2))
	r.SetCtxVar(middleware.CtxKeyApiKeyID, int64(3))

	_, err := selectTaskChannel(r, []byte(`{"model":"video-model"}`))
	if !errors.Is(err, relay_common.ErrApiKeyModelNotAllowed) {
		t.Fatalf("selectTaskChannel() error = %v, want %v", err, relay_common.ErrApiKeyModelNotAllowed)
	}
	if checker.apiKeyChecks != 1 {
		t.Fatalf("API key checks = %d, want 1", checker.apiKeyChecks)
	}
	if checker.tenantChecks != 0 {
		t.Fatalf("tenant model checks = %d, want 0 after authorization denial", checker.tenantChecks)
	}
}

func TestTaskChannelSelectionError_ApiKeyModelDenied(t *testing.T) {
	statusCode, errType, message := taskChannelSelectionError(relay_common.ErrApiKeyModelNotAllowed)

	if statusCode != 403 {
		t.Fatalf("status code = %d, want 403", statusCode)
	}
	if errType != "permission_denied" {
		t.Fatalf("error type = %q, want permission_denied", errType)
	}
	if message != "当前 API Key 无权使用该模型" {
		t.Fatalf("message = %q, want API Key model denial message", message)
	}
}
