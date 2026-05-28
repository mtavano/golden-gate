package config

import _ "embed"

// DefaultServiceJSON is the canonical starter service.json shipped inside the
// binary. On first boot, if the configured CONFIG_PATH does not exist, this
// content is written to that path so the user can edit it on the persistent
// volume and the changes survive future redeploys.
//
//go:embed default_service.json
var DefaultServiceJSON []byte
