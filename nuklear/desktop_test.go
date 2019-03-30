package nuklear

import (
	"context"
	"testing"

	"github.com/raedahgroup/godcr/app"
)

func TestLaunchApp(t *testing.T) {
	tests := []struct {
		name             string
		ctx              context.Context
		walletMiddleware app.WalletMiddleware
		wantErr          bool
	}{
		// TODO: Add test cases.
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := LaunchApp(test.ctx, test.walletMiddleware); (err != nil) != test.wantErr {
				t.Errorf("LaunchApp() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
