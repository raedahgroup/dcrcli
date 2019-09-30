package nuklear

import (
	"fmt"
	"image"
	"os"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/rect"
	"github.com/raedahgroup/dcrlibwallet"
	"github.com/raedahgroup/godcr/nuklear/nuklog"
	"github.com/raedahgroup/godcr/nuklear/styles"
	"github.com/raedahgroup/godcr/nuklear/widgets"
)

const (
	navWidth            = 200
	defaultWindowWidth  = 800
	defaultWindowHeight = 600
)

type nuklearApp struct {
	appDisplayName string
	wallet         *dcrlibwallet.LibWallet
	navPages       map[string]navPageHandler
	currentPage    string
	nextPage       string
	pageChanged    bool
}

func LaunchUserInterface(appDisplayName, appDataDir, netType string) {
	logger, err := dcrlibwallet.RegisterLogger("NUKL")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Launch error - cannot register logger: %v", err)
		return
	}

	nuklog.UseLogger(logger)

	app := &nuklearApp{}

	app.wallet, err = dcrlibwallet.NewLibWallet(appDataDir, "", netType)
	if err != nil {
		nuklog.Log.Errorf("Initialization error: %v", err)
		return
	}

	walletExists, err := app.wallet.WalletExists()
	if err != nil {
		nuklog.Log.Errorf("Error checking if wallet db exists: %v", err)
		return
	}

	if !walletExists {
		// todo show create wallet page
		nuklog.Log.Infof("Wallet does not exist in app directory. Need to create one.")
		return
	}

	var pubPass []byte
	if app.wallet.ReadBoolConfigValueForKey(dcrlibwallet.IsStartupSecuritySetConfigKey) {
		// prompt user for public passphrase and assign to `pubPass`
	}

	err = app.wallet.OpenWallet(pubPass)
	if err != nil {
		nuklog.Log.Errorf("Error opening wallet db: %v", err)
		return
	}

	err = app.wallet.SpvSync("")
	if err != nil {
		nuklog.Log.Errorf("Spv sync attempt failed: %v", err)
		return
	}

	// initialize master window and set style
	windowSize := image.Point{X: defaultWindowWidth, Y: defaultWindowHeight}
	masterWindow := nucular.NewMasterWindowSize(nucular.WindowNoScrollbar, appDisplayName, windowSize, app.render)
	masterWindow.SetStyle(styles.MasterWindowStyle())

	// initialize fonts for later use
	err = styles.InitFonts()
	if err != nil {
		nuklog.Log.Errorf("Error initializing app fonts: %v", err)
		return
	}

	// register nav page handlers
	navPages := getNavPages()
	app.navPages = make(map[string]navPageHandler, len(navPages))
	for _, page := range navPages {
		app.navPages[page.name] = page.handler
	}

	// todo: start sync progress listener in background and update current page display as appropriate
	//go app.syncer.startSyncing(walletMiddleware, masterWindow)

	app.currentPage = "overview"
	app.pageChanged = true

	// draw master window
	masterWindow.Main()
}

func (desktop *nuklearApp) render(window *nucular.Window) {
	if _, isNavPage := desktop.navPages[desktop.currentPage]; isNavPage {
		desktop.renderNavPage(window)
	} else {
		errorMessage := fmt.Sprintf("Page not properly set up: %s", desktop.currentPage)
		nuklog.Log.Errorf(errorMessage)

		w := &widgets.Window{window}
		w.DisplayMessage(errorMessage, styles.DecredOrangeColor)
	}
}

func (desktop *nuklearApp) renderNavPage(window *nucular.Window) {
	// this creates the space on the window that will hold 2 widgets
	// the navigation section on the window and the main page content
	entireWindow := window.Row(0).SpaceBegin(2)

	desktop.renderNavWindow(window, entireWindow.H)
	desktop.renderPageContentWindow(window, entireWindow.W, entireWindow.H)
}

func (desktop *nuklearApp) renderNavWindow(window *nucular.Window, maxHeight int) {
	navSectionRect := rect.Rect{
		X: 0,
		Y: 0,
		W: navWidth,
		H: maxHeight,
	}
	window.LayoutSpacePushScaled(navSectionRect)

	// set style
	styles.SetNavStyle(window.Master())

	// create window and draw nav menu
	widgets.NoScrollGroupWindow("nav-group-window", window, func(navGroupWindow *widgets.Window) {
		navGroupWindow.AddHorizontalSpace(10)
		navGroupWindow.AddColoredLabel(fmt.Sprintf("%s %s", desktop.appDisplayName, desktop.wallet.NetType()),
			styles.DecredLightBlueColor, widgets.CenterAlign)
		navGroupWindow.AddHorizontalSpace(10)

		for _, page := range getNavPages() {
			if desktop.currentPage == page.name {
				navGroupWindow.AddCurrentNavButton(page.label, func() {
					desktop.changePage(window, page.name)
				})
			} else {
				navGroupWindow.AddBigButton(page.label, func() {
					desktop.changePage(window, page.name)
				})
			}
		}

		// add exit button
		navGroupWindow.AddBigButton("Exit", func() {
			go navGroupWindow.Master().Close()
		})
	})
}

func (desktop *nuklearApp) renderPageContentWindow(window *nucular.Window, maxWidth, maxHeight int) {
	pageSectionRect := rect.Rect{
		X: navWidth,
		Y: 0,
		W: maxWidth - navWidth,
		H: maxHeight,
	}

	// set style
	styles.SetPageStyle(window.Master())
	window.LayoutSpacePushScaled(pageSectionRect)

	handler := desktop.navPages[desktop.currentPage]
	// ensure that the handler's BeforeRender function is called only once per page call
	// as it initializes page variables
	if desktop.pageChanged {
		handler.BeforeRender(desktop.wallet, window.Master().Changed)
		desktop.pageChanged = false
	}
	handler.Render(window)
}

func (desktop *nuklearApp) changePage(window *nucular.Window, newPage string) {
	desktop.nextPage = newPage
	desktop.currentPage = newPage
	desktop.pageChanged = true
	window.Master().Changed()
}
