package daemon

import (
	"context"
	"net"
	"net/http"
)

// unixDialer returns an http.Transport DialContext that connects over a unix socket.
func unixDialer(sock string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", sock)
		},
	}
}
