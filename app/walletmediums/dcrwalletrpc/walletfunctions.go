package dcrwalletrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/raedahgroup/dcrlibwallet"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrdata/txhelpers"
	"github.com/decred/dcrwallet/rpc/walletrpc"
	"github.com/raedahgroup/dcrlibwallet/txhelper"
	"github.com/raedahgroup/godcr/app/walletcore"
)

// ideally, we should let user provide this info in settings and use the user provided value
// using a constant now to make it easier to update the code where this value is required/used
const requiredConfirmations = 0

func (c *WalletRPCClient) AccountBalance(accountNumber uint32) (*walletcore.Balance, error) {
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

func (c *WalletRPCClient) AccountsOverview() ([]*walletcore.Account, error) {
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

func (c *WalletRPCClient) NextAccount(accountName string, passphrase string) (uint32, error) {
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

func (c *WalletRPCClient) AccountNumber(accountName string) (uint32, error) {
	req := &walletrpc.AccountNumberRequest{
		AccountName: accountName,
	}

	r, err := c.walletService.AccountNumber(context.Background(), req)
	if err != nil {
		return 0, err
	}

	return r.AccountNumber, nil
}

func (c *WalletRPCClient) GenerateReceiveAddress(account uint32) (string, error) {
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

func (c *WalletRPCClient) ValidateAddress(address string) (bool, error) {
	req := &walletrpc.ValidateAddressRequest{
		Address: address,
	}

	validationResult, err := c.walletService.ValidateAddress(context.Background(), req)
	if err != nil {
		return false, err
	}

	return validationResult.IsValid, nil
}

func (c *WalletRPCClient) UnspentOutputs(account uint32, targetAmount int64) ([]*walletcore.UnspentOutput, error) {
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

func (c *WalletRPCClient) GenerateChangeAddresses(sourceAccount uint32, nChangeOutputs, nInputs int, totalInputAmount int64, destinations []txhelper.TransactionDestination) ([]string, int64, error) {
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

func (c *WalletRPCClient) SendFromAccount(sourceAccount uint32, destinations []txhelper.TransactionDestination, passphrase string) (string, error) {
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

func (c *WalletRPCClient) SendFromUTXOs(sourceAccount uint32, utxoKeys []string, destinations []txhelper.TransactionDestination, passphrase string) (string, error) {
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

func (c *WalletRPCClient) TransactionHistory() ([]*walletcore.Transaction, error) {
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

func (c *WalletRPCClient) GetTransaction(transactionHash string) (*walletcore.TransactionDetails, error) {
	hash, err := chainhash.NewHashFromStr(transactionHash)
	if err != nil {
		return nil, fmt.Errorf("invalid hash: %s\n%s", transactionHash, err.Error())
	}
	getTxRequest := &walletrpc.GetTransactionRequest{TransactionHash: hash[:]}
	getTxResponse, err := c.walletService.GetTransaction(context.Background(), getTxRequest)
	if err != nil {
		return nil, err
	}

	transactionHex := fmt.Sprintf("%x", getTxResponse.GetTransaction().GetTransaction())
	msgTx, err := txhelpers.MsgTxFromHex(transactionHex)
	if err != nil {
		return nil, err
	}

	transaction, err := processTransaction(getTxResponse.GetTransaction())
	if err != nil {
		return nil, err
	}
	txFee, txFeeRate := txhelpers.TxFeeRate(msgTx)
	transaction.Fee, transaction.Rate, transaction.Size = txFee, txFeeRate, msgTx.SerializeSize()

	credits := getTxResponse.GetTransaction().GetCredits()
	txOutputs, err := outputsFromMsgTxOut(msgTx.TxOut, credits, c.activeNet)
	if err != nil {
		return nil, err
	}
	return &walletcore.TransactionDetails{
		BlockHash:     fmt.Sprintf("%x", getTxResponse.GetBlockHash()),
		Confirmations: getTxResponse.GetConfirmations(),
		Transaction:   transaction,
		Inputs:        inputsFromMsgTxIn(msgTx.TxIn),
		Outputs:       txOutputs,
	}, nil
}

func (c *WalletRPCClient) StakeInfo(ctx context.Context) (*walletcore.StakeInfo, error) {
	stakeInfoResponse, err := c.walletService.StakeInfo(ctx, &walletrpc.StakeInfoRequest{})
	if err != nil {
		return nil, err
	}

	totalTickets := stakeInfoResponse.OwnMempoolTix +
		stakeInfoResponse.Live + stakeInfoResponse.Immature + stakeInfoResponse.Unspent

	ticketsResponse, err := c.walletService.GetTickets(ctx, &walletrpc.GetTicketsRequest{})
	if err != nil {
		return nil, err
	}

	stakeInfo := &walletcore.StakeInfo{
		Immature:      stakeInfoResponse.Immature,
		Live:          stakeInfoResponse.Live,
		OwnMempoolTix: stakeInfoResponse.OwnMempoolTix,
		Tickets:       make([]walletcore.Ticket, 0),
		Total:         totalTickets,
		Unspent:       stakeInfoResponse.Unspent,
	}

	for {
		response, err := ticketsResponse.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		ticketHash := response.GetTicket().GetTicket().GetHash()
		ticketStatus := response.GetTicket().GetTicketStatus().String()
		hash, err := chainhash.NewHash(ticketHash)
		if err != nil {
			return nil, err
		}
		stakeInfo.Tickets = append(stakeInfo.Tickets, walletcore.Ticket{Hash: hash.String(), Status: ticketStatus})
	}

	return stakeInfo, err
}

func (c *WalletRPCClient) PurchaseTicket(ctx context.Context, request dcrlibwallet.PurchaseTicketsRequest) (ticketHashes []string, err error) {
	amount, err := dcrutil.NewAmount(float64(request.SpendLimit))
	if err != nil {
		return nil, fmt.Errorf("invalid amount for spend limit: %s", err.Error())
	}
	request.SpendLimit = int64(amount)
	priceResponse, err := c.walletService.TicketPrice(ctx, &walletrpc.TicketPriceRequest{})
	if err != nil {
		return nil, fmt.Errorf("could not determine ticket ticketPrice: %s", err.Error())
	}
	ticketPrice := priceResponse.GetTicketPrice()
	if request.SpendLimit < ticketPrice {
		return nil, fmt.Errorf("insufficient funds: spend limit %v is less that ticket ticketPrice %v", amount, ticketPrice)
	}
	balance, err := c.AccountBalance(request.Account)
	if err != nil {
		return nil, fmt.Errorf("could not fetch account: %v", err.Error())
	}
	if balance.Spendable < dcrutil.Amount(ticketPrice) {
		return nil, fmt.Errorf("insufficient funds: account balance %v is less than ticket price %v", balance.Spendable, ticketPrice)
	}
	response, err := c.walletService.PurchaseTickets(ctx, &walletrpc.PurchaseTicketsRequest{
		Account:               request.Account,
		Expiry:                request.Expiry,
		NumTickets:            request.NumTickets,
		Passphrase:            request.Passphrase,
		PoolAddress:           request.PoolAddress,
		PoolFees:              request.PoolFees,
		RequiredConfirmations: request.RequiredConfirmations,
		SpendLimit:            request.SpendLimit,
		TicketAddress:         request.TicketAddress,
		TicketFee:             request.TicketFee,
		TxFee:                 request.TxFee,
	})
	if err != nil {
		return nil, fmt.Errorf("error purchasing tickets: %s", err.Error())
	}
	ticketHashes = make([]string, len(response.GetTicketHashes()))
	for i, ticketHash := range response.GetTicketHashes() {
		hash, err := chainhash.NewHash(ticketHash)
		if err != nil {
			return ticketHashes, fmt.Errorf("error purchasing tickets: %s", err.Error())
		}
		ticketHashes[i] = hash.String()
	}
	return ticketHashes, nil
}
