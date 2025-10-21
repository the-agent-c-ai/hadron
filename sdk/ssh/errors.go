package ssh

import "errors"

// ErrConnectionClose indicates failure closing SSH connection.
var ErrConnectionClose = errors.New("failed to close SSH connection")
