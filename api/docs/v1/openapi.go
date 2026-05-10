package v1

import "github.com/gogf/gf/v2/frame/g"

// OpenAPISpecReq returns the OpenAPI 3.0 specification for /v1/* endpoints.
type OpenAPISpecReq struct {
	g.Meta `path:"/docs/api/openapi.json" method:"get" tags:"Docs" summary:"Get OpenAPI 3.0 spec"`
}

type OpenAPISpecRes struct {
	// Spec is returned as raw JSON; controller writes it directly.
}
