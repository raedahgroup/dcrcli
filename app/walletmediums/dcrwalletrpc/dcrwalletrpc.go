package dcrwalletrpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/decred/dcrwallet/netparams"
	"github.com/decred/dcrwallet/rpc/walletrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// WalletPRCClient implements `WalletMiddleware` using `mobilewallet.LibWallet` as medium for connecting to a decred wallet
// Functions relating to operations that can be performed on a wallet are defined in `walletfunctions.go`
// Other wallet-related functions are defined in `walletloader.go`
type WalletPRCClient struct {
	walletLoader  walletrpc.WalletLoaderServiceClient
	walletService walletrpc.WalletServiceClient
	netType       string
	walletOpen    bool
}

type rpcConnectionResult struct {
	err  error
	conn *grpc.ClientConn
}

var (
	rpcConnectionDone    = make(chan *rpcConnectionResult)
	rpcConnectionTimeout = 5 * time.Second
)

func New(ctx context.Context, netType, rpcAddress, rpcCert string, noTLS bool) (*WalletPRCClient, error) {
	if rpcAddress == "" {
		rpcAddress = defaultDcrWalletRPCAddress(netType)
	}

	// perform rpc connection in background, user might shutdown before connection is complete
	go connectToRPC(rpcAddress, rpcCert, noTLS)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case connectionResult := <-rpcConnectionDone:
		if connectionResult.err != nil {
			return nil, connectionResult.err
		}

		client := &WalletPRCClient{
			walletLoader:  walletrpc.NewWalletLoaderServiceClient(connectionResult.conn),
			walletService: walletrpc.NewWalletServiceClient(connectionResult.conn),
			netType:       netType,
		}

		return client, nil
	}
}

func defaultDcrWalletRPCAddress(netType string) string {
	if netType == "mainnet" {
		return net.JoinHostPort("localhost", netparams.MainNetParams.GRPCServerPort)
	} else {
		return net.JoinHostPort("localhost", netparams.TestNet3Params.GRPCServerPort)
	}
}

func connectToRPC(rpcAddress, rpcCert string, noTLS bool) {
	var conn *grpc.ClientConn
	var err error

	defer func() {
		if conn == nil && err == nil {
			// connection timeout
			err = fmt.Errorf("Error connecting to %s. Connection attempt timed out after %s", rpcAddress, rpcConnectionTimeout)
		}
		connectionResult := &rpcConnectionResult{
			err:  err,
			conn: conn,
		}
		rpcConnectionDone <- connectionResult
	}()

	// block until connection is established
	// return error if connection cannot be established after `rpcConnectionTimeoutSeconds` seconds
	grpcConnectionOptions := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTimeout(rpcConnectionTimeout),
	}

	if noTLS {
		grpcConnectionOptions = append(grpcConnectionOptions, grpc.WithInsecure())
		conn, err = grpc.Dial(rpcAddress, grpcConnectionOptions...)
	} else {
		creds, err := credentials.NewClientTLSFromFile(rpcCert, "")
		if err != nil {
			return
		}

		grpcConnectionOptions = append(grpcConnectionOptions, grpc.WithTransportCredentials(creds))
		conn, err = grpc.Dial(rpcAddress, grpcConnectionOptions...)
	}
}
