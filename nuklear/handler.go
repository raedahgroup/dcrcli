package nuklear

import (
	"github.com/aarzilli/nucular"
	"github.com/raedahgroup/godcr/app"
	"github.com/raedahgroup/godcr/app/walletcore"
	"github.com/raedahgroup/godcr/nuklear/pagehandlers"
	"github.com/raedahgroup/godcr/nuklear/pagehandlers/transaction"
	"github.com/raedahgroup/godcr/nuklear/styles"
	"github.com/raedahgroup/godcr/nuklear/widgets"
)

type standalonePage struct {
	name    string
	handler standalonePageHandler
}

type standalonePageHandler interface {
	BeforeRender(walletMiddleware app.WalletMiddleware, refreshWindowDisplay func())
	Render(*nucular.Window, func(*nucular.Window, string))
}

type navPage struct {
	name    string
	label   string
	handler navPageHandler
}

type navPageHandler interface {
	// BeforeRender initializes all variables that will be needed for displaying the page.
	// It is expected that this method will only be called once i.e. when the page is switched to from a different page.
	// It might be necessary to load some wallet data in background thread
	// after which the app can be notified to repaint the page using `refreshWindowDisplay()`
	// Returns true when done.
	BeforeRender(wallet walletcore.Wallet, refreshWindowDisplay func()) bool

	// Render draws widgets on the provided window.
	// It is usually called several times not only when the page is navigated to.
	// For example, this method will be triggered whenever the mouse is moved, causing the window to repaint.
	Render(window *nucular.Window)
}

func getStandalonePages() []standalonePage {
	return []standalonePage{
		{
			name:    "sync",
			handler: &pagehandlers.SyncHandler{},
		},
		{
			name:    "createwallet",
			handler: &pagehandlers.CreateWalletHandler{},
		},
	}
}

func getNavPages() []navPage {
	return []navPage{
		{
			name:    "overview",
			label:   "Overview",
			handler: &pagehandlers.OverviewHandler{},
		},
		{
			name:    "history",
			label:   "History",
			handler: &transaction.HistoryHandler{},
		},
		{
			name:    "send",
			label:   "Send",
			handler: &pagehandlers.SendHandler{},
		},
		{
			name:    "receive",
			label:   "Receive",
			handler: &pagehandlers.ReceiveHandler{},
		},
		{
			name:    "staking",
			label:   "Staking",
			handler: &pagehandlers.StakingHandler{},
		},
		{
			name:    "accounts",
			label:   "Accounts",
			handler: &notImplementedNavPageHandler{"Accounts"},
		},
		{
			name:    "security",
			label:   "Security",
			handler: &notImplementedNavPageHandler{"Security"},
		},
		{
			name:    "settings",
			label:   "Settings",
			handler: &notImplementedNavPageHandler{"Settings"},
		},
	}
}

type notImplementedNavPageHandler struct {
	pageTitle string
}

func (_ *notImplementedNavPageHandler) BeforeRender(_ walletcore.Wallet, _ func()) bool {
	return true
}

func (p *notImplementedNavPageHandler) Render(window *nucular.Window) {
	widgets.PageContentWindowDefaultPadding(p.pageTitle, window, func(contentWindow *widgets.Window) {
		contentWindow.DisplayMessage("Page not yet implemented", styles.GrayColor)
	})
}
