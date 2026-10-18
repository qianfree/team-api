package packed

import _ "embed"

//go:embed setup.html
var SetupHTML []byte

//go:embed setup.js
var SetupJS []byte

//go:embed landing.html
var LandingHTML []byte

//go:embed landing.js
var LandingJS []byte
