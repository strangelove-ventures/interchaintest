package lib

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/api/info"
	"go.uber.org/zap"
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

func WaitNode(ctx context.Context, host, port string, logger *zap.Logger, nodeIndex int, blockchainID string) error {
	err := WaitPort(ctx, host, port)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second)

	client := info.NewClient(fmt.Sprintf("http://%s:%s", host, port))
	for done := false; !done && err == nil; {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context closed")
		default:
			var subnetDone bool
			var subnetErr error
			xdone, xerr := client.IsBootstrapped(ctx, "X")
			if errors.Is(xerr, io.EOF) || errors.Is(xerr, syscall.ECONNREFUSED) {
				xerr = nil
			}
			pdone, perr := client.IsBootstrapped(ctx, "P")
			if errors.Is(perr, io.EOF) || errors.Is(perr, syscall.ECONNREFUSED) {
				perr = nil
			}
			cdone, cerr := client.IsBootstrapped(ctx, "C")
			if errors.Is(cerr, io.EOF) || errors.Is(cerr, syscall.ECONNREFUSED) {
				cerr = nil
			}
			if blockchainID != "" {
				subnetDone, subnetErr = client.IsBootstrapped(ctx, blockchainID)
				if errors.Is(subnetErr, io.EOF) || errors.Is(subnetErr, syscall.ECONNREFUSED) {
					subnetErr = nil
				}
			}
			fields := []zap.Field{
				zap.Int("node", nodeIndex),
				zap.Boolp("x", &xdone),
				zap.Boolp("p", &pdone),
				zap.Boolp("c", &cdone),
			}
			if blockchainID != "" {
				fields = append(fields, zap.Boolp("subnet", &subnetDone))
			}
			logger.Info(
				"bootstrap status",
				fields...,
			)

			done = xdone && pdone && cdone
			err = errors.Join(xerr, perr, cerr)
			if blockchainID != "" {
				done = done && subnetDone
				err = errors.Join(err, subnetErr)
			}
			time.Sleep(5 * time.Second)
		}
	}

	return err
}
