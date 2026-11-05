package commstransport

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DialUnixContext is unavailable on Windows. The existing named-pipe API is
// separate; a Unix caller never falls back to it or to another endpoint.
func DialUnixContext(context.Context, string, time.Duration) (net.Conn, error) {
	return nil, fmt.Errorf("comms Unix transport is unavailable on Windows")
}
