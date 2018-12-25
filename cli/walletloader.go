package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/raedahgroup/dcrcli/app"
	"github.com/raedahgroup/dcrcli/cli/terminalprompt"
)

// createWallet creates a new wallet if one doesn't already exist using the WalletMiddleware provided
func createWallet(walletMiddleware app.WalletMiddleware) {
	// first check if wallet already exists
	walletExists, err := walletMiddleware.WalletExists()
	if err != nil {
		errMsg := fmt.Sprintf("Error checking %s wallet", walletMiddleware.NetType())
		printErrorAndExit(errMsg, err)
	}
	if walletExists {
		netType := strings.Title(walletMiddleware.NetType())
		errMsg := fmt.Sprintf("%s wallet already exists", netType)
		printErrorAndExit(errMsg, nil)
	}

	// ask user to enter passphrase twice
	passphrase, err := terminalprompt.RequestInputSecure("Enter private passphrase for new wallet", terminalprompt.EmptyValidator)
	if err != nil {
		printErrorAndExit("Error reading input", err)
	}
	confirmPassphrase, err := terminalprompt.RequestInputSecure("Confirm passphrase", terminalprompt.EmptyValidator)
	if err != nil {
		printErrorAndExit("Error reading input", err)
	}
	if passphrase != confirmPassphrase {
		printErrorAndExit("Passphrases do not match", nil)
	}

	// get seed and display to user
	seed, err := walletMiddleware.GenerateNewWalletSeed()
	if err != nil {
		printErrorAndExit("Error generating seed for new wallet", err)
	}
	displayWalletSeed(seed)

	// ask user to back seed up, only proceed after user does so
	backupPrompt := `Enter "OK" to continue. This assumes you have stored the seed in a safe and secure location`
	backupValidator := func(userResponse string) error {
		userResponse = strings.TrimSpace(userResponse)
		userResponse = strings.Trim(userResponse, `"`)
		if strings.EqualFold("OK", userResponse) {
			return nil
		} else {
			return fmt.Errorf("invalid response, try again")
		}
	}
	_, err = terminalprompt.RequestInput(backupPrompt, backupValidator)
	if err != nil {
		printErrorAndExit("Error reading input", err)
	}

	// user entered "OK" in last prompt, finalize wallet creation
	err = walletMiddleware.CreateWallet(passphrase, seed)
	if err != nil {
		printErrorAndExit("Error creating wallet", err)
	}

	fmt.Println("Your wallet has been created successfully")

	// perform first blockchain sync after creating wallet
	syncBlockChain(walletMiddleware)
}

// openWallet is called whenever an action to be executed requires wallet to be loaded
// exits the program if wallet doesn't exist or some other error occurs
func openWallet(walletMiddleware app.WalletMiddleware) {
	walletExists, err := walletMiddleware.WalletExists()
	if err != nil {
		errMsg := fmt.Sprintf("Error checking %s wallet", walletMiddleware.NetType())
		printErrorAndExit(errMsg, err)
	}

	if !walletExists {
		netType := strings.Title(walletMiddleware.NetType())
		errMsg := fmt.Sprintf("%s wallet does not exist. Create it using '%s --createwallet'", netType, app.Name())
		printErrorAndExit(errMsg, nil)
	}

	err = walletMiddleware.OpenWallet()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to open %s wallet", walletMiddleware.NetType())
		printErrorAndExit(errMsg, err)
	}
}

// syncBlockChain uses the WalletMiddleware provided to download block updates
// causes app to exit if an error is encountered
func syncBlockChain(walletMiddleware app.WalletMiddleware) {
	var err error
	defer func() {
		if err != nil {
			printErrorAndExit("Error syncing blockchain", err)
		} else {
			fmt.Println("Blockchain synced successfully")
		}
	}()

	// use wait group to wait for go routine process to complete before exiting this function
	var wg sync.WaitGroup
	wg.Add(1)

	err = walletMiddleware.SyncBlockChain(&app.BlockChainSyncListener{
		SyncStarted: func() {
			fmt.Println("Starting blockchain sync")
		},
		SyncEnded: func(e error) {
			err = e
			wg.Done()
		},
		OnHeadersFetched:    func(percentageProgress int64) {}, // in cli mode, sync updates are logged to terminal, no need to act on this update alert
		OnDiscoveredAddress: func(state string) {},             // in cli mode, sync updates are logged to terminal, no need to act on update alert
		OnRescanningBlocks:  func(percentageProgress int64) {}, // in cli mode, sync updates are logged to terminal, no need to act on update alert
	})

	if err != nil {
		// sync go routine failed to start, nothing to wait for
		wg.Done()
	} else {
		// sync in progress, wait for BlockChainSyncListener.OnComplete
		wg.Wait()
	}
}
