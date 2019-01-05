package dcrwalletrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrwallet/rpc/walletrpc"
	"github.com/raedahgroup/dcrlibwallet/addresshelper"
	"github.com/raedahgroup/dcrlibwallet/txhelper"
	"github.com/raedahgroup/godcr/app/walletcore"
)

// ideally, we should let user provide this info in settings and use the user provided value
// using a constant now to make it easier to update the code where this value is required/used
const requiredConfirmations = 0

func (c *WalletPRCClient) AccountBalance(accountNumber uint32) (*walletcore.Balance, error) {
	req := &walletrpc.BalanceRequest{
		AccountNumber:         accountNumber,
		RequiredConfirmations: requiredConfirmations,
	}

	res, err := c.walletService.Balance(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("error fetching balance for account: %d \n:%s", accountNumber, err.Error())
	}

	return &walletcore.Balance{
		Total:           dcrutil.Amount(res.Total),
		Spendable:       dcrutil.Amount(res.Spendable),
		LockedByTickets: dcrutil.Amount(res.LockedByTickets),
		VotingAuthority: dcrutil.Amount(res.VotingAuthority),
		Unconfirmed:     dcrutil.Amount(res.Unconfirmed),
	}, nil
}

func (c *WalletPRCClient) AccountsOverview() ([]*walletcore.Account, error) {
	accounts, err := c.walletService.Accounts(context.Background(), &walletrpc.AccountsRequest{})
	if err != nil {
		return nil, fmt.Errorf("error fetching accounts: %s", err.Error())
	}

	accountsOverview := make([]*walletcore.Account, 0, len(accounts.Accounts))

	for _, acc := range accounts.Accounts {
		balance, err := c.AccountBalance(acc.AccountNumber)
		if err != nil {
			return nil, err
		}

		// skip zero-balance imported accounts
		if acc.AccountName == "imported" && balance.Total == 0 {
			continue
		}

		account := &walletcore.Account{
			Name:    acc.AccountName,
			Number:  acc.AccountNumber,
			Balance: balance,
		}
		accountsOverview = append(accountsOverview, account)
	}

	return accountsOverview, nil
}

func (c *WalletPRCClient) NextAccount(accountName string, passphrase string) (uint32, error) {
	req := &walletrpc.NextAccountRequest{
		AccountName: accountName,
		Passphrase:  []byte(passphrase),
	}

	nextAccount, err := c.walletService.NextAccount(context.Background(), req)
	if err != nil {
		return 0, err
	}

	return nextAccount.AccountNumber, nil
}

func (c *WalletPRCClient) AccountNumber(accountName string) (uint32, error) {
	req := &walletrpc.AccountNumberRequest{
		AccountName: accountName,
	}

	r, err := c.walletService.AccountNumber(context.Background(), req)
	if err != nil {
		return 0, err
	}

	return r.AccountNumber, nil
}

func (c *WalletPRCClient) AccountName(accountNumber uint32) (string, error) {
	accounts, err := c.walletService.Accounts(context.Background(), &walletrpc.AccountsRequest{})
	if err != nil {
		return "", err
	}

	for _, account := range accounts.Accounts {
		if account.AccountNumber == accountNumber {
			return account.AccountName, nil
		}
	}

	return "", fmt.Errorf("Account not found")
}

func (c *WalletPRCClient) AddressInfo(address string) (*txhelper.AddressInfo, error) {
	req := &walletrpc.ValidateAddressRequest{
		Address: address,
	}

	addressValidationResult, err := c.walletService.ValidateAddress(context.Background(), req)
	if err != nil {
		return nil, err
	}

	addressInfo := &txhelper.AddressInfo{
		IsMine: addressValidationResult.IsMine,
		Address: address,
	}
	if addressValidationResult.IsMine {
		addressInfo.AccountNumber = addressValidationResult.AccountNumber
		addressInfo.AccountName, _ = c.AccountName(addressValidationResult.AccountNumber)
	}

	return addressInfo, nil
}

// ValidateAddress tries to decode an address for the given network params, if error is encountered, address is not valid
func (c *WalletPRCClient) ValidateAddress(address string) (bool, error) {
	_, err := addresshelper.DecodeForNetwork(address, c.activeNet)
	return err == nil, nil
}

func (c *WalletPRCClient) GenerateReceiveAddress(account uint32) (string, error) {
	req := &walletrpc.NextAddressRequest{
		Account:   account,
		GapPolicy: walletrpc.NextAddressRequest_GAP_POLICY_WRAP,
		Kind:      walletrpc.NextAddressRequest_BIP0044_EXTERNAL,
	}

	nextAddress, err := c.walletService.NextAddress(context.Background(), req)
	if err != nil {
		return "", err
	}

	return nextAddress.Address, nil
}

func (c *WalletPRCClient) UnspentOutputs(account uint32, targetAmount int64) ([]*walletcore.UnspentOutput, error) {
	utxoStream, err := c.unspentOutputStream(account, targetAmount, requiredConfirmations)
	if err != nil {
		return nil, err
	}

	var unspentOutputs []*walletcore.UnspentOutput

	for {
		utxo, err := utxoStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		hash, err := chainhash.NewHash(utxo.TransactionHash)
		if err != nil {
			return nil, err
		}
		txHash := hash.String()

		unspentOutput := &walletcore.UnspentOutput{
			OutputKey:       fmt.Sprintf("%s:%d", txHash, utxo.OutputIndex),
			TransactionHash: txHash,
			OutputIndex:     utxo.OutputIndex,
			Tree:            utxo.Tree,
			ReceiveTime:     utxo.ReceiveTime,
			Amount:          dcrutil.Amount(utxo.Amount),
		}
		unspentOutputs = append(unspentOutputs, unspentOutput)
	}

	return unspentOutputs, nil
}

func (c *WalletPRCClient) GenerateChangeAddresses(sourceAccount uint32, nChangeOutputs, nInputs int, totalInputAmount int64, destinations []txhelper.TransactionDestination) ([]string, int64, error) {
	// generate addresses for account
	changeAddresses := make([]string, nChangeOutputs)
	for i := 0; i < nChangeOutputs; i++ {
		address, err := c.GenerateReceiveAddress(sourceAccount)
		if err != nil {
			return nil, 0, err
		}
		changeAddresses[i] = address
	}

	// use generated addresses together with other provided info to estimate change from transaction after subtracting fee
	changeAmount, err := txhelper.EstimateChange(nInputs, totalInputAmount, destinations, changeAddresses)
	if err != nil {
		return nil, 0, err
	}

	return changeAddresses, changeAmount, nil
}

func (c *WalletPRCClient) SendFromAccount(sourceAccount uint32, destinations []txhelper.TransactionDestination, passphrase string) (string, error) {
	// construct non-change outputs for all recipients
	outputs := make([]*walletrpc.ConstructTransactionRequest_Output, len(destinations))
	for i, destination := range destinations {
		amountInAtom, err := txhelper.AmountToAtom(destination.Amount)
		if err != nil {
			return "", err
		}

		outputs[i] = &walletrpc.ConstructTransactionRequest_Output{
			Destination: &walletrpc.ConstructTransactionRequest_OutputDestination{
				Address: destination.Address,
			},
			Amount: amountInAtom,
		}
	}

	// construct transaction
	constructRequest := &walletrpc.ConstructTransactionRequest{
		SourceAccount:         sourceAccount,
		NonChangeOutputs:      outputs,
		RequiredConfirmations: requiredConfirmations,
	}

	constructResponse, err := c.walletService.ConstructTransaction(context.Background(), constructRequest)
	if err != nil {
		return "", fmt.Errorf("error constructing transaction: %s", err.Error())
	}

	return c.signAndPublishTransaction(constructResponse.UnsignedTransaction, passphrase)
}

func (c *WalletPRCClient) SendFromUTXOs(sourceAccount uint32, utxoKeys []string, destinations []txhelper.TransactionDestination, passphrase string) (string, error) {
	// fetch all utxos in account to extract details for the utxos selected by user
	// passing 0 as targetAmount to c.unspentOutputStream fetches ALL utxos in account
	utxoStream, err := c.unspentOutputStream(sourceAccount, 0, requiredConfirmations)
	if err != nil {
		return "", err
	}

	// loop through utxo stream to find user selected utxos
	inputs := make([]*wire.TxIn, 0, len(utxoKeys))
	for {
		utxo, err := utxoStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		transactionHash, err := chainhash.NewHash(utxo.TransactionHash)
		if err != nil {
			return "", fmt.Errorf("invalid utxo transaction hash: %s", err.Error())
		}
		utxoKey := fmt.Sprintf("%s:%d", transactionHash.String(), utxo.OutputIndex)

		useUtxo := false
		for _, key := range utxoKeys {
			if utxoKey == key {
				useUtxo = true
			}
		}
		if !useUtxo {
			continue
		}

		outpoint := wire.NewOutPoint(transactionHash, utxo.OutputIndex, int8(utxo.Tree))
		input := wire.NewTxIn(outpoint, utxo.Amount, nil)
		inputs = append(inputs, input)

		if len(inputs) == len(utxoKeys) {
			break
		}
	}

	// generate address from sourceAccount to receive change
	changeAddress, err := c.GenerateReceiveAddress(sourceAccount)
	if err != nil {
		return "", err
	}

	unsignedTx, err := txhelper.NewUnsignedTx(inputs, destinations, changeAddress)
	if err != nil {
		return "", err
	}

	// serialize unsigned tx
	var txBuf bytes.Buffer
	txBuf.Grow(unsignedTx.SerializeSize())
	err = unsignedTx.Serialize(&txBuf)
	if err != nil {
		return "", fmt.Errorf("error serializing transaction: %s", err.Error())
	}

	return c.signAndPublishTransaction(txBuf.Bytes(), passphrase)
}

func (c *WalletPRCClient) TransactionHistory() ([]*walletcore.Transaction, error) {
	req := &walletrpc.GetTransactionsRequest{}

	stream, err := c.walletService.GetTransactions(context.Background(), req)
	if err != nil {
		return nil, err
	}

	var transactions []*walletcore.Transaction

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		var transactionDetails []*walletrpc.TransactionDetails
		if in.MinedTransactions != nil {
			transactionDetails = append(transactionDetails, in.MinedTransactions.Transactions...)
		}
		if in.UnminedTransactions != nil {
			transactionDetails = append(transactionDetails, in.UnminedTransactions...)
		}

		txs, err := processTransactions(transactionDetails)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, txs...)
	}

	// sort transactions by date (list newer first)
	sort.SliceStable(transactions, func(i1, i2 int) bool {
		return transactions[i1].Timestamp > transactions[i2].Timestamp
	})

	return transactions, nil
}

func (c *WalletPRCClient) GetTransaction(transactionHash string) (*walletcore.TransactionDetails, error) {
	ctx := context.Background()

	hash, err := chainhash.NewHashFromStr(transactionHash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash: %s\n%s", transactionHash, err.Error())
	}
	getTxRequest := &walletrpc.GetTransactionRequest{TransactionHash: hash[:]}
	getTxResponse, err := c.walletService.GetTransaction(ctx, getTxRequest)
	if err != nil {
		return nil, err
	}

	// func to attempt to check if an address belongs to the wallet to retrieve it's account name or return external if not
	getWalletAddressInfo := func(address string) *txhelper.AddressInfo {
		addressInfo, err := c.AddressInfo(address)
		if err != nil {
			return &txhelper.AddressInfo{
				Address: address,
			}
		}
		return addressInfo
	}
	decodedTx, err := txhelper.DecodeTransaction(hash, getTxResponse.GetTransaction().GetTransaction(), c.activeNet, getWalletAddressInfo)
	if err != nil {
		return nil, err
	}

	transaction, err := processTransaction(getTxResponse.GetTransaction())
	if err != nil {
		return nil, err
	}
	transaction.Fee, transaction.FeeRate, transaction.Size = dcrutil.Amount(decodedTx.Fee), dcrutil.Amount(decodedTx.FeeRate), decodedTx.Size

	var blockHeight int32 = -1
	if getTxResponse.BlockHash != nil {
		blockInfo, err := c.walletService.BlockInfo(ctx, &walletrpc.BlockInfoRequest{BlockHash: getTxResponse.BlockHash})
		if err == nil {
			blockHeight = blockInfo.BlockHeight
		}
	}

	return &walletcore.TransactionDetails{
		BlockHeight:   blockHeight,
		Confirmations: getTxResponse.GetConfirmations(),
		Transaction:   transaction,
		Inputs:        decodedTx.Inputs,
		Outputs:       decodedTx.Outputs,
	}, nil
}
