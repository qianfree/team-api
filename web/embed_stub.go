//go:build !embedweb

package web

import "embed"

var AdminFS embed.FS
var AdminMobileFS embed.FS
var TenantFS embed.FS

var Enabled = false
