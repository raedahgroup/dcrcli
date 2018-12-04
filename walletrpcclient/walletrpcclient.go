package walletrpcclient

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/wire"
	pb "github.com/decred/dcrwallet/rpc/walletrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/raedahgroup/dcrcli/walletcore"
)

type Client struct {
	walletServiceClient pb.WalletServiceClient
}

func New(address, cert string, noTLS bool) (*Client, error) {
	c := &Client{}
	conn, err := c.connect(address, cert, noTLS)
	if err != nil {
		return nil, err
	}

	// register clients
	c.walletServiceClient = pb.NewWalletServiceClient(conn)

	return c, nil
}

func (c *Client) connect(address, cert string, noTLS bool) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error

	if noTLS {
		conn, err = grpc.Dial(address, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
	} else {
		creds, err := credentials.NewClientTLSFromFile(cert, "")
		if err != nil {
			return nil, err
		}

		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(
				creds,
			),
		}

		conn, err = grpc.Dial(address, opts...)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}

func (c *Client) SendFromAccount(amountInDCR float64, sourceAccount uint32, destinationAddress,
	passphrase string) (*SendResult, error) {
	
	// convert amount from float64 DCR to int64 Atom as required by dcrwallet ConstructTransaction implementation
	amountInAtom, err := dcrutil.NewAmount(amountInDCR)
	if err != nil {
		return nil, err
	}
	// type of amountInAtom is `dcrutil.Amount` which is an int64 alias
	amount := int64(amountInAtom)
	
	// construct transaction
	pkScript, err := walletcore.GetPKScript(destinationAddress)
	if err != nil {
		return nil, err
	}
	constructRequest := &pb.ConstructTransactionRequest{
		SourceAccount: sourceAccount,
		NonChangeOutputs: []*pb.ConstructTransactionRequest_Output{{
			Destination: &pb.ConstructTransactionRequest_OutputDestination{
				Script:        pkScript,
				ScriptVersion: 0,
			},
			Amount: amount,
		}},
	}

	constructResponse, err := c.walletServiceClient.ConstructTransaction(context.Background(), constructRequest)
	if err != nil {
		return nil, fmt.Errorf("error constructing transaction: %s", err.Error())
	}

	return c.signAndPublishTransaction(constructResponse.UnsignedTransaction, passphrase)
}

func (c *Client) SendFromUTXOs(utxoKeys []string, amountInDCR float64, sourceAccount uint32,
	destinationAddress, passphrase string) (*SendResult, error) {
	
	// convert amount to atoms
	amountInAtom, err := dcrutil.NewAmount(amountInDCR)
	amount := int64(amountInAtom)

	// fetch all utxos to extract details for the utxos selected by user
	req := &pb.UnspentOutputsRequest{
		Account:                  sourceAccount,
		TargetAmount:             0,
		RequiredConfirmations:    0,
		IncludeImmatureCoinbases: true,
	}
	stream, err := c.walletServiceClient.UnspentOutputs(context.Background(), req)
	if err != nil {
		return nil, err
	}

	// loop through utxo stream to find user selected utxos
	inputs := make([]*wire.TxIn, 0, len(utxoKeys))
	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		// todo could _ be an error
		transactionHash, _ := chainhash.NewHash(item.TransactionHash)

		outputKey := fmt.Sprintf("%s:%v", transactionHash.String(), item.OutputIndex)
		useUtxo := false
		for _, key := range utxoKeys {
			if outputKey == key {
				useUtxo = true
			}
		}
		if !useUtxo {
			continue
		}

		outpoint := wire.NewOutPoint(transactionHash, item.OutputIndex, int8(item.Tree));
		input := wire.NewTxIn(outpoint, item.Amount, nil)
		inputs = append(inputs, input)

		// todo compare capacity with length
		if len(inputs) == len(utxoKeys) {
			break;
		}
	}

	// generate address from sourceAccount to receive change
	receiveResult, err := c.Receive(sourceAccount)
	if err != nil {
		return nil, err
	}
	changeAddress := receiveResult.Address

	unsignedTx, err := walletcore.NewUnsignedTx(inputs, amount, destinationAddress, changeAddress)
	if err != nil {
		return nil, err
	}
	
	// serialize unsigned tx
	var txBuf bytes.Buffer
	txBuf.Grow(unsignedTx.SerializeSize())
	err = unsignedTx.Serialize(&txBuf)
	if err != nil {
		return nil, fmt.Errorf("error serializing transaction: %s", err.Error())
	}

	return c.signAndPublishTransaction(txBuf.Bytes(), passphrase)
}

func (c *Client) signAndPublishTransaction(serializedTx []byte, passphrase string) (*SendResult, error) {
	ctx := context.Background()

	// sign transaction
	signRequest := &pb.SignTransactionRequest{
		Passphrase:            []byte(passphrase),
		SerializedTransaction: serializedTx,
	}

	signResponse, err := c.walletServiceClient.SignTransaction(ctx, signRequest)
	if err != nil {
		return nil, fmt.Errorf("error signing transaction: %s", err.Error())
	}

	// publish transaction
	publishRequest := &pb.PublishTransactionRequest{
		SignedTransaction: signResponse.Transaction,
	}

	publishResponse, err := c.walletServiceClient.PublishTransaction(ctx, publishRequest)
	if err != nil {
		return nil, fmt.Errorf("error publishing transaction: %s", err.Error())
	}

	transactionHash, _ := chainhash.NewHash(publishResponse.TransactionHash)

	response := &SendResult{
		TransactionHash: transactionHash.String(),
	}

	return response, nil
}

func (c *Client) Balance() ([]AccountBalanceResult, error) {
	ctx := context.Background()
	accounts, err := c.walletServiceClient.Accounts(ctx, &pb.AccountsRequest{})
	if err != nil {
		return nil, fmt.Errorf("error fetching accounts: %s", err.Error())
	}

	balanceResult := make([]AccountBalanceResult, 0, len(accounts.Accounts))

	for _, v := range accounts.Accounts {
		req := &pb.BalanceRequest{
			AccountNumber:         v.AccountNumber,
			RequiredConfirmations: 0,
		}

		res, err := c.walletServiceClient.Balance(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("error fetching balance for account: %d :%s", v.AccountNumber, err.Error())
		}

		if v.AccountName == "imported" && dcrutil.Amount(res.Total) == 0 {
			continue
		}

		accountBalance := AccountBalanceResult{
			AccountNumber:   v.AccountNumber,
			AccountName:     v.AccountName,
			Total:           dcrutil.Amount(res.Total),
			Spendable:       dcrutil.Amount(res.Spendable),
			LockedByTickets: dcrutil.Amount(res.LockedByTickets),
			VotingAuthority: dcrutil.Amount(res.VotingAuthority),
			Unconfirmed:     dcrutil.Amount(res.Unconfirmed),
		}

		balanceResult = append(balanceResult, accountBalance)
	}

	return balanceResult, nil
}

func (c *Client) Receive(accountNumber uint32) (*ReceiveResult, error) {
	ctx := context.Background()

	req := &pb.NextAddressRequest{
		Account:   accountNumber,
		GapPolicy: pb.NextAddressRequest_GAP_POLICY_WRAP,
		Kind:      pb.NextAddressRequest_BIP0044_EXTERNAL,
	}

	r, err := c.walletServiceClient.NextAddress(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error generating receive address: %s", err.Error())
	}

	res := &ReceiveResult{
		Address: r.Address,
	}

	return res, nil
}

func (c *Client) ValidateAddress(address string) (bool, error) {
	req := &pb.ValidateAddressRequest{
		Address: address,
	}

	r, err := c.walletServiceClient.ValidateAddress(context.Background(), req)
	if err != nil {
		return false, err
	}

	return r.IsValid, nil
}

func (c *Client) NextAccount(accountName string, passphrase string) (uint32, error) {
	req := &pb.NextAccountRequest{
		AccountName: accountName,
		Passphrase:  []byte(passphrase),
	}

	r, err := c.walletServiceClient.NextAccount(context.Background(), req)
	if err != nil {
		return 0, err
	}

	return r.AccountNumber, nil
}

func (c *Client) GetTransaction(txHashStr string) (*TransactionResult, error) {
	txHash, err := chainhash.NewHashFromStr(txHashStr)
	if err != nil {
		return nil, err
	}

	req := &pb.GetTransactionRequest{
		TransactionHash: txHash.CloneBytes(),
	}

	r, err := c.walletServiceClient.GetTransaction(context.Background(), req)
	if err != nil {
		return nil, err
	}

	tx := &TransactionResult{
		Confirmations: r.Confirmations,
		BlockHash:     r.BlockHash,
		TransactionDetails: &transaction{
			Fee:             r.Transaction.Fee,
			Transaction:     r.Transaction.Transaction,
			Timestamp:       r.Transaction.Timestamp,
			TransactionType: int(r.Transaction.TransactionType),
			Input:           make([]*transactionInput, len(r.Transaction.Debits)),
			Output:          make([]*transactionOutput, len(r.Transaction.Credits)),
		},
	}

	for i, v := range r.Transaction.Debits {
		input := &transactionInput{
			Index:           v.Index,
			PreviousAccount: v.PreviousAccount,
			PreviousAmount:  v.PreviousAmount,
		}
		tx.TransactionDetails.Input[i] = input
	}

	for i, v := range r.Transaction.Credits {
		output := &transactionOutput{
			Index:        v.Index,
			Account:      v.Account,
			Internal:     v.Internal,
			Amount:       v.Amount,
			Address:      v.Address,
			OutputScript: v.OutputScript,
		}
		tx.TransactionDetails.Output[i] = output
	}

	return tx, nil
}

func (c *Client) UnspentOutputs(account uint32, targetAmount int64) ([]*UnspentOutputsResult, error) {
	req := &pb.UnspentOutputsRequest{
		Account:                  account,
		TargetAmount:             targetAmount,
		RequiredConfirmations:    0,
		IncludeImmatureCoinbases: true,
	}

	stream, err := c.walletServiceClient.UnspentOutputs(context.Background(), req)
	if err != nil {
		return nil, err
	}

	outputs := []*UnspentOutputsResult{}

	for {
		item, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		transactionHash, _ := chainhash.NewHash(item.TransactionHash)

		outputItem := &UnspentOutputsResult{
			OutputKey:		 fmt.Sprintf("%s:%v", transactionHash.String(), item.OutputIndex),
			TransactionHash: transactionHash.String(),
			OutputIndex:     item.OutputIndex,
			Amount:          AtomToCoin(item.Amount),
			PkScript:        item.PkScript,
			AmountSum:       AtomToCoin(item.AmountSum),
			ReceiveTime:     item.ReceiveTime,
			Tree:            item.Tree,
			FromCoinbase:    item.FromCoinbase,
		}
		outputs = append(outputs, outputItem)
	}

	return outputs, nil
}
