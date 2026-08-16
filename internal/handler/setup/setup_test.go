package setup_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/guid"

	"github.com/qianfree/team-api/internal/consts"
	setuphandler "github.com/qianfree/team-api/internal/handler/setup"
	"github.com/qianfree/team-api/internal/testutil"
)

func TestHandleSetupInitializeValidationErrors(t *testing.T) {
	s := g.Server(guid.S())
	s.BindHandler("/initialize", setuphandler.HandleSetupInitialize)

	baseURL := testutil.StartGFServer(t, s)

	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "invalid username",
			body:     `{"username":"ab","password":"Password1","confirmPassword":"Password1"}`,
			wantCode: consts.CodeSetupInvalidUsername,
		},
		{
			name:     "weak password",
			body:     `{"username":"admin","password":"short","confirmPassword":"short"}`,
			wantCode: consts.CodePasswordTooWeak,
		},
		{
			name:     "password mismatch",
			body:     `{"username":"admin","password":"Password1","confirmPassword":"Password2"}`,
			wantCode: consts.CodeSetupPasswordMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Post(
				baseURL+"/initialize",
				"application/json",
				strings.NewReader(tt.body),
			)
			if err != nil {
				t.Fatalf("POST /initialize: %v", err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want %d", res.StatusCode, http.StatusUnprocessableEntity)
			}

			var body struct {
				Code int `json:"code"`
			}
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", body.Code, tt.wantCode)
			}
		})
	}
}
