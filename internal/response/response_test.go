package response_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/guid"

	apiresponse "github.com/qianfree/team-api/internal/response"
	"github.com/qianfree/team-api/internal/testutil"
)

func TestErrorResponseStatusAndCode(t *testing.T) {
	s := g.Server(guid.S())
	s.BindHandler("/standard", func(r *ghttp.Request) {
		apiresponse.ErrorMsg(r, http.StatusBadRequest, "bad request")
	})
	s.BindHandler("/business", func(r *ghttp.Request) {
		apiresponse.ErrorWithCode(r, http.StatusUnprocessableEntity, 10079, "invalid username")
	})
	s.BindHandler("/invalid-status", func(r *ghttp.Request) {
		apiresponse.ErrorWithCode(r, 10079, 10079, "invalid username")
	})

	baseURL := testutil.StartGFServer(t, s)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantCode   int
	}{
		{
			name:       "standard HTTP error",
			path:       "/standard",
			wantStatus: http.StatusBadRequest,
			wantCode:   http.StatusBadRequest,
		},
		{
			name:       "business error uses separate HTTP status",
			path:       "/business",
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   10079,
		},
		{
			name:       "invalid HTTP status falls back to internal error",
			path:       "/invalid-status",
			wantStatus: http.StatusInternalServerError,
			wantCode:   10079,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Get(baseURL + tt.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer res.Body.Close()

			if res.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.wantStatus)
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
