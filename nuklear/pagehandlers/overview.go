package pagehandlers

import (
	"github.com/aarzilli/nucular"
	"github.com/raedahgroup/godcr/app/config"
	"github.com/raedahgroup/godcr/app/walletcore"
	"github.com/raedahgroup/godcr/nuklear/styles"
	"github.com/raedahgroup/godcr/nuklear/widgets"
)

type OverviewHandler struct {
	err      error
	accounts []*walletcore.Account
}

func (handler *OverviewHandler) BeforeRender(wallet walletcore.Wallet, settings *config.Settings, _ func()) bool {
	handler.accounts, handler.err = wallet.AccountsOverview(walletcore.DefaultRequiredConfirmations)
	return true
}

func (handler *OverviewHandler) Render(window *nucular.Window) {
	widgets.PageContentWindowDefaultPadding("Overview", window, func(contentWindow *widgets.Window) {
		contentWindow.AddLabelWithFont("Current Total Balance", widgets.LeftCenterAlign, styles.BoldPageContentFont)

		if handler.err != nil {
			contentWindow.DisplayErrorMessage("Error fetching accounts balance", handler.err)
		} else {
			contentWindow.AddLabel(walletcore.WalletBalance(handler.accounts), widgets.LeftCenterAlign)
		}
	})
}
