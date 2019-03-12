package pages

import (
	"reflect"
	"testing"

	"github.com/raedahgroup/godcr/app/walletcore"
	"github.com/rivo/tview"
)

func TestHistoryPage(t *testing.T) {
	tests := []struct {
		name       string
		wallet     walletcore.Wallet
		setFocus   func(p tview.Primitive) *tview.Application
		clearFocus func()
		want       tview.Primitive
	}{
		// TODO: Add test cases.
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := HistoryPage(test.wallet, test.setFocus, test.clearFocus); !reflect.DeepEqual(got, test.want) {
				t.Errorf("HistoryPage() = %v, want %v", got, test.want)
			}
		})
	}
}
