package cmd

import (
	"fmt"

	"github.com/dotandev/hintents/internal/rpc"
	"github.com/spf13/cobra"
)

var debugCmd = &cobra.Command{
	Use:   "debug <transaction-hash>",
	Short: "Debug a failed Soroban transaction",
	Long: `Fetch a transaction envelope from the Stellar network and prepare it for simulation.

Example:
  erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
  erst --network testnet debug <tx-hash>`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

		// Create RPC client with the selected network
		client := rpc.NewClient(rpc.Network(NetworkFlag))

		fmt.Printf("Debugging transaction: %s\n", txHash)
		fmt.Printf("Using network: %s\n", NetworkFlag)
		fmt.Printf("Horizon URL: %s\n", client.Horizon.HorizonURL)

		// Fetch transaction (context would be provided by actual implementation)
		// txResp, err := client.GetTransaction(context.Background(), txHash)
		// if err != nil {
		// 	return err
		// }
		// fmt.Printf("Transaction found with envelope XDR size: %d bytes\n", len(txResp.EnvelopeXdr))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
