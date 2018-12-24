package dcrwalletrpc

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"time"

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/txscript"
	"github.com/decred/dcrd/wire"
	"github.com/decred/dcrwallet/netparams"
	"github.com/decred/dcrwallet/rpc/walletrpc"
	"github.com/raedahgroup/dcrcli/walletsource"
)

func amountToAtom(amountInDCR float64) (int64, error) {
	amountInAtom, err := dcrutil.NewAmount(amountInDCR)
	if err != nil {
		return 0, err
	}

	// type of amountInAtom is `dcrutil.Amount` which is an int64 alias
	return int64(amountInAtom), nil
}

func (c *WalletPRCClient) unspentOutputStream(account uint32, targetAmount int64) (walletrpc.WalletService_UnspentOutputsClient, error) {
	req := &walletrpc.UnspentOutputsRequest{
		Account:                  account,
		TargetAmount:             targetAmount,
		RequiredConfirmations:    0,
		IncludeImmatureCoinbases: true,
	}

	return c.walletService.UnspentOutputs(context.Background(), req)
}

func (c *WalletPRCClient) signAndPublishTransaction(serializedTx []byte, passphrase string) (string, error) {
	ctx := context.Background()

	// sign transaction
	signRequest := &walletrpc.SignTransactionRequest{
		Passphrase:            []byte(passphrase),
		SerializedTransaction: serializedTx,
	}

	signResponse, err := c.walletService.SignTransaction(ctx, signRequest)
	if err != nil {
		return "", fmt.Errorf("error signing transaction: %s", err.Error())
	}

	// publish transaction
	publishRequest := &walletrpc.PublishTransactionRequest{
		SignedTransaction: signResponse.Transaction,
	}

	publishResponse, err := c.walletService.PublishTransaction(ctx, publishRequest)
	if err != nil {
		return "", fmt.Errorf("error publishing transaction: %s", err.Error())
	}

	transactionHash, err := chainhash.NewHash(publishResponse.TransactionHash)
	if err != nil {
		return "", fmt.Errorf("error parsing successful transaction hash: %s", err.Error())
	}

	return transactionHash.String(), nil
}

func processTransactions(transactionDetails []*walletrpc.TransactionDetails) ([]*walletsource.Transaction, error) {
	transactions := make([]*walletsource.Transaction, 0, len(transactionDetails))

	for _, txDetail := range transactionDetails {
		// use any of the addresses in inputs/outputs to determine if this is a testnet tx
		var isTestnet bool
		for _, output := range txDetail.Credits {
			isMainNet, err := addressIsForNet(output.GetAddress(), netparams.MainNetParams.Params)
			if err != nil {
				continue
			}
			isTestnet = !isMainNet
			break
		}

		tx, err := processTransaction(txDetail, isTestnet)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, tx)
	}

	return transactions, nil
}

func processTransaction(txDetail *walletrpc.TransactionDetails, isTestnet bool) (*walletsource.Transaction, error) {
	hash, err := chainhash.NewHash(txDetail.Hash)
	if err != nil {
		return nil, err
	}

	amount, direction := transactionAmountAndDirection(txDetail)

	tx := &walletsource.Transaction{
		Hash:          hash.String(),
		Amount:        dcrutil.Amount(amount),
		Fee:           dcrutil.Amount(txDetail.Fee),
		Type:          txDetail.TransactionType.String(),
		Direction:     direction,
		Testnet:       isTestnet,
		Timestamp:     txDetail.Timestamp,
		FormattedTime: time.Unix(txDetail.Timestamp, 0).Format("Mon Jan 2, 2006 3:04PM"),
	}
	return tx, nil
}

func addressIsForNet(address string, net *chaincfg.Params) (bool, error) {
	addr, err := dcrutil.DecodeAddress(address)
	if err != nil {
		return false, err
	}
	return addr.IsForNet(net), nil
}

func transactionAmountAndDirection(txDetail *walletrpc.TransactionDetails) (int64, walletsource.TransactionDirection) {
	var outputAmounts int64
	for _, credit := range txDetail.Credits {
		outputAmounts += int64(credit.Amount)
	}

	var inputAmounts int64
	for _, debit := range txDetail.Debits {
		inputAmounts += int64(debit.PreviousAmount)
	}

	var amount int64
	var direction walletsource.TransactionDirection

	if txDetail.TransactionType == walletrpc.TransactionDetails_REGULAR {
		amountDifference := outputAmounts - inputAmounts
		if amountDifference < 0 && (float64(txDetail.Fee) == math.Abs(float64(amountDifference))) {
			// transfered internally, the only real amount spent was transaction fee
			direction = walletsource.TransactionDirectionTransferred
			amount = int64(txDetail.Fee)
		} else if amountDifference > 0 {
			// received
			direction = walletsource.TransactionDirectionReceived

			for _, credit := range txDetail.Credits {
				amount += int64(credit.Amount)
			}
		} else {
			// sent
			direction = walletsource.TransactionDirectionSent

			for _, debit := range txDetail.Debits {
				amount += int64(debit.PreviousAmount)
			}
			for _, credit := range txDetail.Credits {
				amount -= int64(credit.Amount)
			}
			amount -= int64(txDetail.Fee)
		}
	}

	return amount, direction
}

func inputsFromMsgTxIn(txIn []*wire.TxIn) []*walletsource.TxInput {
	txInputs := make([]*walletsource.TxInput, len(txIn))
	for i, input := range txIn {
		txInputs[i] = &walletsource.TxInput{
			Amount: dcrutil.Amount(input.ValueIn),
			PreviousOutpoint: input.PreviousOutPoint.String(),
		}
	}
	return txInputs
}

func outputsFromMsgTxOut(txOut []*wire.TxOut, walletCredits []*walletrpc.TransactionDetails_Output, chainParams *chaincfg.Params) ([]*walletsource.TxOutput, error) {
	txOutputs := make([]*walletsource.TxOutput, len(txOut))
	for i, output := range txOut {
		_, addrs, _, err := txscript.ExtractPkScriptAddrs(output.Version, output.PkScript, chainParams)
		if err != nil {
			return nil, err
		}
		txOutputs[i] = &walletsource.TxOutput{
			Value: dcrutil.Amount(output.Value),
			Address: addrs[0].String(),
		}
		for _, credit := range walletCredits {
			if bytes.Equal(output.PkScript, credit.GetOutputScript()) {
				txOutputs[i].Internal = credit.GetInternal()
			}
		}
	}
	return txOutputs, nil
}