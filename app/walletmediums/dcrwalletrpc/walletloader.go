package dcrwalletrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/hdkeychain"
	"github.com/decred/dcrwallet/rpc/walletrpc"
	"github.com/decred/dcrwallet/walletseed"
	"github.com/raedahgroup/dcrlibwallet"
	"github.com/raedahgroup/dcrlibwallet/defaultsynclistener"
	"github.com/raedahgroup/godcr/app/walletcore"
)

func (c *WalletRPCClient) GenerateNewWalletSeed() (string, error) {
	seed, err := hdkeychain.GenerateSeed(hdkeychain.RecommendedSeedLen)
	if err != nil {
		return "", err
	}

	return walletseed.EncodeMnemonic(seed), nil
}

func (c *WalletRPCClient) WalletExists() (bool, error) {
	res, err := c.walletLoader.WalletExists(context.Background(), &walletrpc.WalletExistsRequest{})
	if err != nil {
		return false, err
	}
	return res.Exists, nil
}

func (c *WalletRPCClient) CreateWallet(passphrase, seed string) error {
	seedBytes, err := walletseed.DecodeUserInput(seed)
	if err != nil {
		return err
	}

	_, err = c.walletLoader.CreateWallet(context.Background(), &walletrpc.CreateWalletRequest{
		PrivatePassphrase: []byte(passphrase),
		Seed:              seedBytes,
	})

	// wallet will be opened if the create operation was successful
	if err == nil {
		c.walletOpen = true
	}

	return err
}

func (c *WalletRPCClient) IsWalletOpen() bool {
	// for now, assume that the wallet's already open since we're connecting through dcrwallet daemon
	// ideally, we'd have to use dcrwallet's WalletLoaderService to do this
	return c.walletOpen
}

func (c *WalletRPCClient) SyncBlockChain(showLog bool, syncProgressUpdated func(*defaultsynclistener.ProgressReport)) {
	ctx := context.Background() // todo use a cancelable ctx

	getBestBlock := func() int32 {
		bestBlockHeight, _ := c.BestBlock()
		return int32(bestBlockHeight)
	}
	getBestBlockTimestamp := func() int64 {
		var bestBlockInfo *walletrpc.BlockInfoResponse
		bestBlock, err := c.walletService.BestBlock(ctx, &walletrpc.BestBlockRequest{})
		if err == nil {
			bestBlockInfo, err = c.walletService.BlockInfo(ctx, &walletrpc.BlockInfoRequest{BlockHash: bestBlock.Hash})
		}
		if err != nil {
			return 0
		}
		return bestBlockInfo.Timestamp
	}

	// c.syncListener listens for reported sync updates, calculates progress and updates the caller via syncProgressUpdated
	if c.syncListener == nil {
		// use syncProgressUpdatedWrapper to suppress op parameter that's not needed by callers
		syncProgressUpdatedWrapper := func(progressReport *defaultsynclistener.ProgressReport, _ defaultsynclistener.SyncOp) {
			syncProgressUpdated(progressReport)
		}
		c.syncListener = defaultsynclistener.DefaultSyncProgressListener(c.NetType(), showLog, getBestBlock, getBestBlockTimestamp,
			syncProgressUpdatedWrapper)
	}

	syncStream, err := c.walletLoader.SpvSync(ctx, &walletrpc.SpvSyncRequest{})
	if err != nil {
		c.syncListener.OnSyncError(dcrlibwallet.ErrorCodeUnexpectedError, err)
		return
	}

	// read sync updates from syncStream in go routine and trigger c.syncListener methods to calculate progress and update caller
	go func() {
		for {
			syncUpdate, err := syncStream.Recv()
			if err != nil {
				c.syncListener.OnSyncError(dcrlibwallet.ErrorCodeUnexpectedError, err)
				c.syncListener.OnSynced(false)
				return
			}
			if syncUpdate.Synced {
				c.indexTransactions(ctx, -1, -1, showLog, func() {
					c.syncListener.OnSynced(true)
				})
				return
			}

			switch syncUpdate.NotificationType {
			case walletrpc.SyncNotificationType_FETCHED_HEADERS_STARTED:
				c.syncListener.OnFetchedHeaders(0, 0, dcrlibwallet.SyncStateStart)

			case walletrpc.SyncNotificationType_FETCHED_HEADERS_PROGRESS:
				c.syncListener.OnFetchedHeaders(syncUpdate.FetchHeaders.FetchedHeadersCount, syncUpdate.FetchHeaders.LastHeaderTime,
					dcrlibwallet.SyncStateProgress)

			case walletrpc.SyncNotificationType_FETCHED_HEADERS_FINISHED:
				c.syncListener.OnFetchedHeaders(0, 0, dcrlibwallet.SyncStateFinish)

			case walletrpc.SyncNotificationType_DISCOVER_ADDRESSES_STARTED:
				c.syncListener.OnDiscoveredAddresses(dcrlibwallet.SyncStateStart)

			case walletrpc.SyncNotificationType_DISCOVER_ADDRESSES_FINISHED:
				c.syncListener.OnDiscoveredAddresses(dcrlibwallet.SyncStateFinish)

			case walletrpc.SyncNotificationType_RESCAN_STARTED:
				c.syncListener.OnRescan(0, dcrlibwallet.SyncStateStart)

			case walletrpc.SyncNotificationType_RESCAN_PROGRESS:
				c.syncListener.OnRescan(syncUpdate.RescanProgress.RescannedThrough, dcrlibwallet.SyncStateProgress)

			case walletrpc.SyncNotificationType_RESCAN_FINISHED:
				c.syncListener.OnRescan(0, dcrlibwallet.SyncStateFinish)

			case walletrpc.SyncNotificationType_PEER_CONNECTED:
				c.numberOfPeers = syncUpdate.PeerInformation.PeerCount
				c.syncListener.OnPeerConnected(syncUpdate.PeerInformation.PeerCount)

			case walletrpc.SyncNotificationType_PEER_DISCONNECTED:
				c.numberOfPeers = syncUpdate.PeerInformation.PeerCount
				c.syncListener.OnPeerConnected(syncUpdate.PeerInformation.PeerCount)
			}
		}
	}()
}

func (c *WalletRPCClient) RescanBlockChain() error {
	if c.syncListener == nil {
		return fmt.Errorf("blockchain has not been synced previously")
	}

	rescanStream, err := c.walletService.Rescan(context.Background(), &walletrpc.RescanRequest{BeginHeight: 0})
	if err != nil {
		return err
	}

	// notify rescan start
	c.syncListener.OnRescan(0, dcrlibwallet.SyncStateStart)

	// read sync updates from rescanStream in goroutine and trigger c.syncListener methods to calculate progress and update caller
	go func() {
		for {
			rescanResponse, err := rescanStream.Recv()
			if err != nil {
				c.syncListener.OnRescan(0, dcrlibwallet.SyncStateFinish)
				return
			}

			// notify rescan progress
			c.syncListener.OnRescan(rescanResponse.RescannedThrough, dcrlibwallet.SyncStateProgress)

			bestBlock, err := c.BestBlock()
			if err == nil && rescanResponse.RescannedThrough >= int32(bestBlock) {
				c.syncListener.OnRescan(0, dcrlibwallet.SyncStateFinish)
				return
			}
		}
	}()

	return nil
}

func (c *WalletRPCClient) WalletConnectionInfo() (info walletcore.ConnectionInfo, err error) {
	accounts, loadAccountErr := c.AccountsOverview(walletcore.DefaultRequiredConfirmations)
	if loadAccountErr != nil {
		err = fmt.Errorf("error fetching account balance: %s", loadAccountErr.Error())
		info.TotalBalance = "0 DCR"
	} else {
		var totalBalance dcrutil.Amount
		for _, acc := range accounts {
			totalBalance += acc.Balance.Total
		}
		info.TotalBalance = totalBalance.String()
	}

	bestBlock, bestBlockErr := c.BestBlock()
	if bestBlockErr != nil && err != nil {
		err = fmt.Errorf("%s, error in fetching best block %s", err.Error(), bestBlockErr.Error())
	} else if bestBlockErr != nil {
		err = bestBlockErr
	}

	info.LatestBlock = bestBlock
	info.NetworkType = c.NetType()
	info.PeersConnected = c.numberOfPeers

	return
}

func (c *WalletRPCClient) BestBlock() (uint32, error) {
	req, err := c.walletService.BestBlock(context.Background(), &walletrpc.BestBlockRequest{})
	if err != nil {
		return 0, err
	}
	return req.Height, err
}

func (c *WalletRPCClient) CloseWallet() {
	// don't actually close wallet loaded by dcrwallet
	// - if wallet wasn't opened by godcr, closing it could cause troubles for user
	// - even if wallet was opened by godcr, closing it without closing dcrwallet would cause troubles for user
	// when they next launch godcr

	// close tx index db though, so it can be re-opened next time
	if c.txIndexDB != nil {
		err := c.txIndexDB.Close()
		if err != nil {
			fmt.Printf("close tx index db error: %s.\n", err.Error())
		}
	}
}

func (c *WalletRPCClient) DeleteWallet() error {
	return errors.New("wallet cannot be deleted when connecting via dcrwallet rpc")
}
