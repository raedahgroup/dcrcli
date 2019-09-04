package walletcore

import (
	"context"

	"github.com/raedahgroup/dcrlibwallet"
	"github.com/raedahgroup/dcrlibwallet/txhelper"
	"github.com/raedahgroup/dcrlibwallet/txindex"
)

// Wallet defines key functions for performing operations on a decred wallet
// These functions are implemented by the different mediums that provide access to a decred wallet
type Wallet interface {
	// Balance returns account balance for the accountNumbers passed in
	// or for all accounts if no account number is passed in
	AccountBalance(accountNumber uint32, requiredConfirmations int32) (*Balance, error)

	// AccountsOverview returns the name, account number and balance for all accounts in wallet
	AccountsOverview(requiredConfirmations int32) ([]*Account, error)

	// NextAccount adds an account to the wallet using the specified name
	// Returns account number for newly added account
	NextAccount(accountName string, passphrase string) (uint32, error)

	// AccountNumber looks up and returns an account number by the account's unique name
	AccountNumber(accountName string) (uint32, error)

	// AccountNumber returns the name for an account  with the provided account number
	AccountName(accountNumber uint32) (string, error)

	// AddressInfo checks if an address belongs to the wallet to retrieve it's account name
	AddressInfo(address string) (*dcrlibwallet.AddressInfo, error)

	// ValidateAddress checks if an address is valid or not
	ValidateAddress(address string) (bool, error)

	// ReceiveAddress checks if there's a previously generated address that hasn't been used to receive funds and returns it
	// If no unused address exists, it generates a new address to receive funds into specified account
	ReceiveAddress(account uint32) (string, error)

	// GenerateNewAddress generates a new address to receive funds into specified account
	// regardless of whether there was a previously generated address that has not been used
	GenerateNewAddress(account uint32) (string, error)

	// UnspentOutputs lists all unspent outputs in the specified account that sum up to `targetAmount`
	// If `targetAmount` is 0, all unspent outputs in account are returned
	UnspentOutputs(account uint32, targetAmount int64, requiredConfirmations int32) ([]*UnspentOutput, error)

	// SendFromAccount sends funds to 1 or more destination addresses, each with a specified amount.
	// The inputs to the transaction are automatically selected from any combination of unspent outputs in the account.
	// Returns the transaction hash as string if successful.
	SendFromAccount(sourceAccount uint32, requiredConfirmations int32, destinations []txhelper.TransactionDestination, passphrase string) (string, error)

	// SendFromUTXOs sends funds to 1 or more destination addresses, each with a specified amount.
	// The inputs to the transaction are unspent outputs in the account, matching the keys sent in []utxoKeys.
	// Also supports specifying how and where to send any change amount that arises from the transaction.
	// If no change destinations are provided, one is automatically created using an address generated from the account.
	// Returns the transaction hash as string if successful
	SendFromUTXOs(sourceAccount uint32, requiredConfirmations int32, utxoKeys []string, txDestinations []txhelper.TransactionDestination, changeDestinations []txhelper.TransactionDestination, passphrase string) (string, error)

	// TransactionCount returns the number of transactions in the tx index database.
	// If `filter` is set to `nil`, all transactions are counted.
	// Otherwise, only transactions matching the provided filter are counted.
	// A `filter` can be created using `txIndex.Filter()`.
	// Fields to filter can be specified using `filter.WithTxTypes(...tx types to return)`
	// and `filter.ForDirections(...tx directions to return)`.
	// Can combine both filters using `filter.WithTxTypes(...tx types to return).ForDirections(...tx directions to return)`.
	TransactionCount(filter *txindex.ReadFilter) (int, error)

	// TransactionHistory fetches the specified count of transactions from a tx index database,
	// beginning at the specified offset.
	// If `filter` is set to `nil`, all transactions are returned.
	// Otherwise, only transactions matching the provided filter are returned.
	// A `filter` can be created using `txIndex.Filter()`.
	// Fields to filter can be specified using `filter.WithTxTypes(...tx types to return)`
	// and `filter.ForDirections(...tx directions to return)`.
	// Can combine both filters using `filter.WithTxTypes(...tx types to return).ForDirections(...tx directions to return)`.
	TransactionHistory(offset, count int32, filter *txindex.ReadFilter) ([]*Transaction, error)

	// GetTransaction returns information about the transaction with the given hash.
	// An error is returned if the no transaction with the given hash is found.
	GetTransaction(transactionHash string) (*Transaction, error)

	// StakeInfo returns information about wallet stakes, tickets and their statuses.
	StakeInfo(ctx context.Context) (*StakeInfo, error)

	// PurchaseTicket is used to purchase tickets.
	PurchaseTicket(ctx context.Context, request dcrlibwallet.PurchaseTicketsRequest) (ticketHashes []string, err error)

	// TicketPrice returns the current ticket price
	TicketPrice(ctx context.Context) (ticketPrice int64, err error)

	// GetTickets returns tickets known to belong to the wallet
	// todo: not yet implemented for dcrwallet rpc
	GetTickets() ([]*dcrlibwallet.TicketInfo, error)

	// ChangePrivatePassphrase changes the private passphrase from the oldPass to the provided newPass
	ChangePrivatePassphrase(ctx context.Context, oldPass, newPass string) error

	// NetType returns the network type of this wallet
	NetType() string
}
