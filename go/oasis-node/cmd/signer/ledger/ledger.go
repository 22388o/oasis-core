// Package ledger implements the ledger signer sub-commands.
package ledger

import (
	"github.com/spf13/cobra"

	ledgerCommon "github.com/oasisprotocol/oasis-core/go/common/ledger"
	cmdCommon "github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common"
)

var (
	ledgerCmd = &cobra.Command{
		Use:   "ledger",
		Short: "interact with Ledger devices",
	}

	listCmd = &cobra.Command{
		Use:   "list_devices",
		Short: "list available devices by address",
		Run:   doLedgerList,
	}
)

func doLedgerList(cmd *cobra.Command, args []string) {
	if err := cmdCommon.Init(); err != nil {
		cmdCommon.EarlyLogAndExit(err)
	}
	ledgerCommon.ListDevices()
}

func Register(parentCmd *cobra.Command) {
	for _, v := range []*cobra.Command{
		listCmd,
	} {
		ledgerCmd.AddCommand(v)
	}

	parentCmd.AddCommand(ledgerCmd)
}
