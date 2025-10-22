package firewall

import "errors"

// ErrParseDefaults indicates failed to parse UFW default policies.
var ErrParseDefaults = errors.New("failed to parse ufw defaults")
