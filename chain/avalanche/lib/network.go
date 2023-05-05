package lib

import (
	"context"
	"fmt"
	"net"
	"time"
)

func IsOpened(host string, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
	if err != nil {
		return false
	}

	if conn != nil {
		conn.Close()
		return true
	}

	return false
}

func WaitPort(ctx context.Context, host, port string) error {
	var err error
	for done := false; !done && err == nil; {
		select {
		case <-ctx.Done():
			err = fmt.Errorf("WaitPort(%s, %s) context closed", host, port)
		default:
			done = IsOpened(host, port)
		}
	}
	return err
}
