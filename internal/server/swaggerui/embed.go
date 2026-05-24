// Package swaggerui embeds vendored Swagger UI v5.32.6 static assets.
// Assets are served locally so no CDN requests are made at runtime.
package swaggerui

import "embed"

//go:embed assets/swagger-ui.css assets/swagger-ui-bundle.js assets/swagger-ui-standalone-preset.js
var Assets embed.FS
