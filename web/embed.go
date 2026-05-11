//go:build embedweb

package web

import "embed"

//go:embed all:admin/dist
var AdminFS embed.FS

//go:embed all:tenant/dist
var TenantFS embed.FS

var Enabled = true
