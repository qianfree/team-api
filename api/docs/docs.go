// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package docs

import (
	"context"

	"github.com/qianfree/team-api/api/docs/v1"
)

type IDocsV1 interface {
	OpenAPISpec(ctx context.Context, req *v1.OpenAPISpecReq) (res *v1.OpenAPISpecRes, err error)
}
