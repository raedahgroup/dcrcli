package cli

import (
	"fmt"
	rpcclient "github.com/raedahgroup/dcrcli/walletrpcclient"
	"github.com/skip2/go-qrcode"
)

func balance(walletrpcclient *rpcclient.Client, commandArgs []string) (*response, error) {
	balances, err := walletrpcclient.Balance()
	if err != nil {
		return nil, err
	}

	res := &response{
		columns: []string{
			"Account",
			"Total",
			"Spendable",
			"Locked By Tickets",
			"Voting Authority",
			"Unconfirmed",
		},
		result: make([][]interface{}, len(balances)),
	}
	for i, v := range balances {
		res.result[i] = []interface{}{
			v.AccountName,
			v.Total,
			v.Spendable,
			v.LockedByTickets,
			v.VotingAuthority,
			v.Unconfirmed,
		}
	}

	return res, nil
}

func normalSend(walletrpcclient *rpcclient.Client, _ []string) (*response, error) {
	return send(walletrpcclient, false)
}

func customSend(walletrpcclient *rpcclient.Client, _ []string) (*response, error) {
	return send(walletrpcclient, true)
}

func send(walletrpcclient *rpcclient.Client, custom bool) (*response, error) {
	var err error

	sourceAccount, err := getSendSourceAccount(walletrpcclient)
	if err != nil {
		return nil, err
	}

	// check if account has positive non-zero balance before proceeding
	// if balance is zero, there'd be no unspent outputs to use
	accountBalance, err := walletrpcclient.SingleAccountBalance(sourceAccount, nil)
	if err != nil {
		return nil, err
	}
	if accountBalance.Total == 0 {
		return nil, fmt.Errorf("Selected account has 0 balance. Cannot proceed")
	}

	destinationAddress, err := getSendDestinationAddress(walletrpcclient)
	if err != nil {
		return nil, err
	}

	sendAmount, err := getSendAmount()
	if err != nil {
		return nil, err
	}

	var utxoSelection []string
	if custom {
		// get all utxos in account, pass 0 amount to get all
		utxos, err := walletrpcclient.UnspentOutputs(sourceAccount, 0)
		if err != nil {
			return nil, err
		}

		utxoSelection, err = getUtxosForNewTransaction(utxos, sendAmount)
		if err != nil {
			return nil, err
		}
	}

	passphrase, err := getWalletPassphrase()
	if err != nil {
		return nil, err
	}

	var result *rpcclient.SendResult
	if custom {
		result, err = walletrpcclient.SendFromUTXOs(utxoSelection, sendAmount, sourceAccount,
			destinationAddress, passphrase)
	} else {
		result, err = walletrpcclient.SendFromAccount(sendAmount, sourceAccount, destinationAddress, passphrase)
	}

	if err != nil {
		return nil, err
	}

	res := &response{
		columns: []string{
			"Result",
			"Hash",
		},
		result: [][]interface{}{
			[]interface{}{
				"The transaction was published successfully",
				result.TransactionHash,
			},
		},
	}

	return res, nil
}

func receive(walletrpcclient *rpcclient.Client, commandArgs []string) (*response, error) {
	var accountNumber uint32

	// if no account name was passed in
	if len(commandArgs) == 0 {
		// display menu options to select account
		var err error
		accountNumber, err = getSendSourceAccount(walletrpcclient)
		if err != nil {
			return nil, err
		}
	} else {
		// if an account name was passed in e.g. ./dcrcli receive default
		// get the address corresponding to the account name and use it
		var err error
		accountNumber, err = walletrpcclient.AccountNumber(commandArgs[0])
		if err != nil {
			return nil, fmt.Errorf("Error fetching account number: %s", err.Error())
		}
	}

	r, err := walletrpcclient.Receive(accountNumber)
	if err != nil {
		return nil, err
	}

	qr, err := qrcode.New(r.Address, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("Error generating QR Code: %s", err.Error())
	}

	res := &response{
		columns: []string{
			"Address",
			"QR Code",
		},
		result: [][]interface{}{
			[]interface{}{
				r.Address,
				qr.ToString(true),
			},
		},
	}
	return res, nil
}

func transactionHistory(walletrpcclient *rpcclient.Client, _ []string) (*response, error) {
	transactions, err := walletrpcclient.GetTransactions()
	if err != nil {
		return nil, err
	}

	res := &response{
		columns: []string{
			"Date",
			"Amount (DCR)",
			"Direction",
			"Hash",
			"Type",
		},
		result: make([][]interface{}, len(transactions)),
	}

	for i, tx := range transactions {
		res.result[i] = []interface{}{
			tx.FormattedTime,
			tx.Amount,
			tx.Direction,
			tx.Hash,
			tx.Type,
		}
	}

	return res, nil
}
