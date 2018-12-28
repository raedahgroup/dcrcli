package dcrwalletrpc

import (
	"context"
	"fmt"
	"time"

	"github.com/decred/dcrd/hdkeychain"
	"github.com/decred/dcrwallet/rpc/walletrpc"
	"github.com/decred/dcrwallet/walletseed"
	"github.com/raedahgroup/dcrcli/app"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *WalletPRCClient) NetType() string {
	return c.netType
}

func (c *WalletPRCClient) WalletExists() (bool, error) {
	res, err := c.walletLoader.WalletExists(context.Background(), &walletrpc.WalletExistsRequest{})
	if err != nil {
		return false, err
	}
	return res.Exists, nil
}

func (c *WalletPRCClient) GenerateNewWalletSeed() (string, error) {
	seed, err := hdkeychain.GenerateSeed(hdkeychain.RecommendedSeedLen)
	if err != nil {
		return "", err
	}

	return walletseed.EncodeMnemonic(seed), nil
}

func (c *WalletPRCClient) CreateWallet(passphrase, seed string) error {
	// since we're connecting through dcrwallet daemon, assume that the wallet's already been created
	// calling create again should return an error
	// ideally, we'd have to use dcrwallet's WalletLoaderService to do this
	return fmt.Errorf("wallet should already be created by dcrwallet daemon")
}

// ignore wallet already open errors, it could be that dcrwallet loaded the wallet when it was launched by the user
// or dcrcli opened the wallet without closing it
func (c *WalletPRCClient) OpenWallet() error {
	_, err := c.walletLoader.OpenWallet(context.Background(), &walletrpc.OpenWalletRequest{})
	if err != nil {
		if e, ok := status.FromError(err); ok && e.Code() == codes.AlreadyExists {
			// wallet already open
			return nil
		}
		return err
	}
	return nil
}

// don't actually close dcrwallet
// - if wallet wasn't opened by dcrcli, closing it could cause troubles for user
// - even if wallet was opened by dcrcli, closing it without closing dcrwallet would cause troubles for user when they next launch dcrcli
func (c *WalletPRCClient) CloseWallet() {
	walletClosed := make(chan bool)

	// walletLoader.CloseWallet causes program to exit abruptly, run in separate goroutine
	go func() {
		time.Sleep(500 * time.Millisecond)
		//c.walletLoader.CloseWallet(context.Background(), &walletrpc.CloseWalletRequest{})
		walletClosed <- true
	}()

	<-walletClosed
	fmt.Println("Wallet closed")
}

func (c *WalletPRCClient) IsWalletOpen() bool {
	// for now, assume that the wallet's already open since we're connecting through dcrwallet daemon
	// ideally, we'd have to use dcrwallet's WalletLoaderService to do this
	return true
}

func (c *WalletPRCClient) SyncBlockChain(listener *app.BlockChainSyncListener, showLog bool) error {
	ctx := context.Background()

	bestBlock, err := c.walletService.BestBlock(ctx, &walletrpc.BestBlockRequest{})
	if err != nil {
		return err
	}

	syncStream, err := c.walletLoader.SpvSync(ctx, &walletrpc.SpvSyncRequest{})
	if err != nil {
		return err
	}

	// create wrapper around success listener and call rpc SubscribeToBlockNotifications
	// method associates the wallet with the consensus RPC server, subscribes the wallet for attached block and chain switch notifications,
	// and causes the wallet to process these notifications in the background.
	// also publish any pending transactions using PublishUnminedTransactions
	originalSyncEndedListener := listener.SyncEnded
	listener.SyncEnded = func(err error) {
		if err != nil {
			_, err := c.walletLoader.SubscribeToBlockNotifications(ctx, &walletrpc.SubscribeToBlockNotificationsRequest{})
			if err != nil {
				// no point pubslishing if above function did not succeed
				c.walletService.PublishUnminedTransactions(ctx, &walletrpc.PublishUnminedTransactionsRequest{})
			}
		}
		originalSyncEndedListener(err)
	}

	s := &spvSync{
		listener:listener,
		netType:c.netType,
		client:syncStream,
		bestBlock:int64(bestBlock.Height),
	}

	// receive sync updates from stream and send to listener in separate goroutine
	go s.streamBlockchainSyncUpdates(showLog)
	return nil
}
