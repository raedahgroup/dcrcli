package routes

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/raedahgroup/dcrcli/app"
)

type syncStatus uint8
const (
	syncStatusNotStarted syncStatus = iota
	syncStatusSuccess
	syncStatusError
	syncStatusInProgress
)

type Blockchain struct {
	sync.RWMutex
	_status syncStatus
	_report string
}

func (routes *Routes) walletLoaderMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return routes.walletLoaderFn(next)
	}
}

// walletLoaderFn checks if wallet is not open, attempts to open it and also perform sync the blockchain
// an error page is displayed and the actual route handler is not called, if ...
// - an error occurs while opening wallet or syncing blockchain
// - wallet is open but blockchain isn't synced
func (routes *Routes) walletLoaderFn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		walletOpen := routes.walletMiddleware.IsWalletOpen()

		// wallet is not open, attempt to open wallet and sync blockchain
		if !walletOpen {
			err := routes.loadWalletAndSyncBlockchain()
			if err != nil {
				routes.renderError(err.Error(), res)
				return
			}
		}

		// wallet is open, check if blockchain is synced
		blockchainSyncStatus := routes.blockchain.status()
		switch blockchainSyncStatus {
		case syncStatusSuccess:
			next.ServeHTTP(res, req)
		case syncStatusNotStarted:
			routes.renderError("Cannot display page. Blockchain hasn't been synced", res)
		case syncStatusInProgress:
			msg := fmt.Sprintf("%s. Refresh after a while to access this page", routes.blockchain.report())
			routes.renderError(msg, res)
		case syncStatusError:
			msg := fmt.Sprintf("Cannot display page. %s", routes.blockchain.report())
			routes.renderError(msg, res)
		default:
			routes.renderError("Cannot display page. Blockchain sync status cannot be determined", res)
		}
	})
}

func (routes *Routes) loadWalletAndSyncBlockchain() error {
	walletExists, err := routes.walletMiddleware.WalletExists()
	if err != nil {
		return fmt.Errorf("Error checking for wallet: %s", err.Error())
	}

	if !walletExists {
		return fmt.Errorf("Wallet not created. Please create a wallet to continue. Use `dcrcli create` on terminal")
	}

	err = routes.walletMiddleware.OpenWallet()
	if err != nil {
		return fmt.Errorf("Failed to open wallet: %s", err.Error())
	}

	routes.syncBlockchain()
	return nil
}

func (routes *Routes) syncBlockchain() {
	updateStatus := routes.blockchain.updateStatus

	err := routes.walletMiddleware.SyncBlockChain(&app.BlockChainSyncListener{
		SyncStarted: func() {
			updateStatus("Starting blockchain sync...", syncStatusInProgress)
		},
		SyncEnded: func(err error) {
			if err != nil {
				updateStatus(fmt.Sprintf("Blockchain sync completed with error: %s", err.Error()), syncStatusError)
			} else {
				updateStatus("Blockchain sync completed successfully", syncStatusSuccess)
			}
		},
		OnHeadersFetched:    func(percentageProgress int64) {
			updateStatus(fmt.Sprintf("Blockchain sync in progress. Fetching headers (1/3): %d%%", percentageProgress), syncStatusInProgress)
		},
		OnDiscoveredAddress: func(_ string) {
			updateStatus("Blockchain sync in progress. Discovering addresses (2/3)", syncStatusInProgress)
		},
		OnRescanningBlocks:  func(percentageProgress int64) {
			updateStatus(fmt.Sprintf("Blockchain sync in progress. Rescanning blocks (3/3): %d%%", percentageProgress), syncStatusInProgress)
		},
	}, false)

	if err != nil {
		updateStatus(fmt.Sprintf("Blockchain sync error: %s", err.Error()), syncStatusError)
	}
}

func (b *Blockchain) updateStatus(report string, status syncStatus) {
	b.Lock()
	b._status = status
	b._report = report
	b.Unlock()
}

func (b *Blockchain) status() syncStatus {
	b.RLock()
	defer b.RUnlock()
	return b._status
}

func (b *Blockchain) report() string {
	b.RLock()
	defer b.RUnlock()
	return b._report
}
