package commands

import (
	"context"
	"github.com/raedahgroup/godcr/app"
	"github.com/raedahgroup/godcr/app/walletcore"
	"github.com/raedahgroup/godcr/cli/termio"
	"strings"
)

type PurchaseTicketCommand struct {
	commanderStub
	MinConfirmations uint32  `long:"min-conf" default:"1" description:"The number of required confirmations for funds used to purchase a ticket." long-description:"If set to zero, it will use unconfirmed and confirmed outputs to purchase tickets."`
	TicketAddress    string  `long:"ticket-address" description:"The address to give voting rights to." long-description:"If it is set to an empty string, an internal address will be used from the wallet."`
	NumTickets       uint32  `long:"num-tickets" default:"1" description:"The number of tickets to purchase."`
	PoolAddress      string  `long:"pool-address" description:"The address of the stake pool used. Pool mode will be disabled if an empty string is passed."`
	PoolFees         float64 `long:"pool-fees" description:"The stake pool fees amount." long-description:"This must be set to a positive value in the allowed range of 0.01 to 100.00 to be valid. It must be set when the pool-address is also set."`
	Expiry           uint32  `long:"expiry" default:"0" description:"The height at which the tickets expire and can no longer enter the blockchain. It defaults to 0 (no expiry)."`
	TxFee            int64   `long:"tx-fee" description:"Fees per kB to use for the transaction generating outputs to use for buying tickets." long-description:"If 0 is passed, the global value for a transaction fee will be used."`
	TicketFee        int64   `description:"Fees per kB to use for all purchased tickets." long-description:"If 0 is passed, the global value for a ticket fee will be used."`
	Args             struct {
		FromAccount string `required:"yes" positional-arg-name:"from account"`
		SpendLimit  int64  `required:"yes" positional-arg-name:"spend limit"`
	} `positional-args:"yes"`
}

func (ptc PurchaseTicketCommand) Run(ctx context.Context, middleware app.WalletMiddleware) error {
	passphrase, err := getWalletPassphrase()
	if err != nil {
		return err
	}
	account, err := middleware.AccountNumber(ptc.Args.FromAccount)
	if err != nil {
		return err
	}
	tickets, err := middleware.PurchaseTicket(ctx, walletcore.PurchaseTicketRequest{
		TxFee:            ptc.TxFee,
		TicketFee:        ptc.TicketFee,
		TicketAddress:    ptc.TicketAddress,
		SpendLimit:       ptc.Args.SpendLimit,
		MinConfirmations: ptc.MinConfirmations,
		PoolFees:         ptc.PoolFees,
		PoolAddress:      ptc.PoolAddress,
		Passphrase:       []byte(passphrase),
		NumTickets:       ptc.NumTickets,
		Expiry:           ptc.Expiry,
		FromAccount:      account,
	})
	if err != nil {
		return err
	}
	output := strings.Builder{}
	for _, ticketHash := range tickets {
		output.WriteString(ticketHash+"\n")
	}
	termio.PrintStringResult(strings.TrimSpace(output.String()))
	return nil
}
