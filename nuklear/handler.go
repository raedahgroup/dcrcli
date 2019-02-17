package nuklear

import (
	"github.com/aarzilli/nucular"
	"github.com/raedahgroup/godcr/app"
	"github.com/raedahgroup/godcr/nuklear/handlers"
)

type Handler interface {
	BeforeRender()
	Render(*nucular.Window, app.WalletMiddleware)
}

type handlersData struct {
	name     string
	navLabel string
	handler  Handler
}

func getHandlers() []handlersData {
	return []handlersData{
		{
			name:     "balance",
			navLabel: "Balance",
			handler:  &handlers.BalanceHandler{},
		},
		{
			name:     "receive",
			navLabel: "Receive",
			handler:  &handlers.ReceiveHandler{},
		},
		{
			name:     "send",
			navLabel: "Send (WIP)",
			handler:  &handlers.SendHandler{},
		},
		{
			name:     "history",
			navLabel: "History",
			handler:  &handlers.TransactionsHandler{},
		},
		{
			name:     "stakeinfo",
			navLabel: "Stake Info",
			handler:  &handlers.StakeInfoHandler{},
		},
		{
			name:     "purchasetickets",
			navLabel: "Purchase Tickets",
			handler:  &handlers.PurchaseTicketsHandler{},
		},
		{
			name:     "createwallet",
			navLabel: "Create Wallet",
			handler:  &handlers.CreateWalletHandler{},
		},
	}
}
