package geyser_grpc

import (
	"context"
	"crypto/x509"
	"errors"
	"net/url"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
)

type gRPCConnOptions struct {
	Target             string
	ClientParameters   *keepalive.ClientParameters
	UseGzipCompression bool
}

func createAndObserveGRPCConn(ctx context.Context, ch chan error, options gRPCConnOptions) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption
	u, err := url.Parse(options.Target)
	if err != nil {
		return nil, err
	}

	port := u.Port()
	if port == "" {
		port = "443"
	}

	if u.Scheme == "http" {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		pool, _ := x509.SystemCertPool()
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(pool, "")))
	}

	hostname := u.Hostname()
	if hostname == "" {
		return nil, errors.New("please provide URL format endpoint e.g. http(s)://<endpoint>:<port>")
	}

	address := hostname + ":" + port

	if options.ClientParameters == nil {
		opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}))
	} else {
		opts = append(opts, grpc.WithKeepaliveParams(*options.ClientParameters))
	}

	if options.UseGzipCompression {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	}

	conn, err := grpc.NewClient(address, opts...)
	if err != nil {
		return nil, err
	}

	go monitorConnectionState(ctx, conn, ch, address, opts...)

	return conn, nil
}

func monitorConnectionState(ctx context.Context, conn *grpc.ClientConn, ch chan error, target string, opts ...grpc.DialOption) {
	var retries int
	for {
		var err error
		select {
		case <-ctx.Done():
			if err := conn.Close(); err != nil {
				ch <- err
			}
			return
		default:
			state := conn.GetState()
			if state == connectivity.Ready {
				retries = 0
				time.Sleep(1 * time.Second)
				continue
			}

			if state == connectivity.TransientFailure || state == connectivity.Connecting || state == connectivity.Idle {
				if retries < 5 {
					time.Sleep(time.Duration(retries) * time.Second)
					conn.ResetConnectBackoff()
					retries++
				} else {
					conn.Close()
					conn, err = grpc.NewClient(target, opts...)
					if err != nil {
						ch <- err
					}
					retries = 0
				}
			} else if state == connectivity.Shutdown {
				conn, err = grpc.NewClient(target, opts...)
				if err != nil {
					ch <- err
				}
				retries = 0
			}

			if !conn.WaitForStateChange(ctx, state) {
				continue
			}
		}
	}
}
