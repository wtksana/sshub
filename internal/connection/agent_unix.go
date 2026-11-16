//go:build linux || darwin

package connection

import (
	"context"
	"errors"
	"net"
	"os"
)

func dialAgent(ctx context.Context) (net.Conn, error) {
	path := os.Getenv("SSH_AUTH_SOCK")
	if path == "" {
		return nil, errors.New("SSH_AUTH_SOCK 未设置")
	}
	return (&net.Dialer{}).DialContext(ctx, "unix", path)
}
