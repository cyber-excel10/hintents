package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Network types for Stellar
type Network string

const (
	Testnet   Network = "testnet"
	Mainnet   Network = "mainnet"
	Futurenet Network = "futurenet"
)

// Global flag variables
var (
	NetworkFlag string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "erst",
	Short: "Erst - Soroban Error Decoder & Debugger",
	Long: `Erst is a specialized developer tool for the Stellar network,
designed to solve the "black box" debugging experience on Soroban.

Use the --network flag to specify which Stellar network to use.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate network flag
		if NetworkFlag != "" {
			switch Network(NetworkFlag) {
			case Testnet, Mainnet, Futurenet:
				// Valid network
			default:
				return fmt.Errorf("invalid network: %s. Must be one of: testnet, mainnet, futurenet", NetworkFlag)
			}
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Add the --network flag to the root command
	rootCmd.PersistentFlags().StringVar(
		&NetworkFlag,
		"network",
		string(Mainnet),
		"Stellar network to use (testnet, mainnet, futurenet)",
	)
}
