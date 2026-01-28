package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"


	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	networkFlag string
	rpcURLFlag  string
)

var debugCmd = &cobra.Command{
	Use:   "debug <transaction-hash>",
	Short: "Debug a failed Soroban transaction",
	Long: `Fetch a transaction envelope from the Stellar network and prepare it for simulation.

Example:
  erst debug 5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab
  erst debug --network testnet <tx-hash>`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// Validate network flag
		switch rpc.Network(networkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			return nil
		default:
			return fmt.Errorf("invalid network: %s. Must be one of: testnet, mainnet, futurenet", networkFlag)
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		txHash := args[0]

		var client *rpc.Client
		if rpcURLFlag != "" {
			client = rpc.NewClientWithURL(rpcURLFlag, rpc.Network(networkFlag))
		} else {
			client = rpc.NewClient(rpc.Network(networkFlag))
		}

		fmt.Printf("Debugging transaction: %s\n", txHash)
		fmt.Printf("Network: %s\n", networkFlag)
		if rpcURLFlag != "" {
			fmt.Printf("RPC URL: %s\n", rpcURLFlag)
		}

		// Fetch transaction details
		resp, err := client.GetTransaction(cmd.Context(), txHash)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		fmt.Printf("Transaction fetched successfully. Envelope size: %d bytes\n", len(resp.EnvelopeXdr))

		// Run simulation
		runner, err := simulator.NewRunner()
		if err != nil {
			return fmt.Errorf("failed to create simulator runner: %w", err)
		}

		simReq := &simulator.SimulationRequest{
			EnvelopeXdr:   resp.EnvelopeXdr,
			ResultMetaXdr: resp.ResultMetaXdr,
		}

		simResp, err := runner.Run(simReq)
		if err != nil {
			return fmt.Errorf("simulation failed: %w", err)
		}

		fmt.Printf("Simulation completed. Status: %s\n", simResp.Status)

		// Save to DB
		db, err := simulator.OpenDB()
		if err != nil {
			fmt.Printf("Warning: failed to open sessions database: %v\n", err)
		} else {
			eventsJSON, _ := json.Marshal(simResp.Events)
			logsJSON, _ := json.Marshal(simResp.Logs)

			session := &simulator.Session{
				TxHash:    txHash,
				Network:   networkFlag,
				Timestamp: time.Now(),
				Error:     simResp.Error,
				Events:    string(eventsJSON),
				Logs:      string(logsJSON),
			}

			if err := db.SaveSession(session); err != nil {
				fmt.Printf("Warning: failed to save session: %v\n", err)
			} else {
				fmt.Println("Session saved to history.")
			}
		}

		return nil
	},
}

var (
	searchError    string
	searchEvent    string
	searchContract string
	searchRegex    bool
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search past debugging sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := simulator.OpenDB()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		filters := simulator.SearchFilters{
			Error:    searchError,
			Event:    searchEvent,
			Contract: searchContract,
			UseRegex: searchRegex,
		}

		sessions, err := db.SearchSessions(filters)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Println("No matching sessions found.")
			return nil
		}

		fmt.Printf("Found %d matching sessions:\n\n", len(sessions))
		for _, s := range sessions {
			fmt.Printf("[%s] %s | Network: %s\n", s.Timestamp.Format("2006-01-02 15:04:05"), s.TxHash, s.Network)
			if s.Error != "" {
				fmt.Printf("  Error: %s\n", s.Error)
			}
			fmt.Println(strings.Repeat("-", 40))
		}

		return nil
	},
}

func init() {
	debugCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	debugCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")

	searchCmd.Flags().StringVar(&searchError, "error", "", "Filter by error message")
	searchCmd.Flags().StringVar(&searchEvent, "event", "", "Search within diagnostic events")
	searchCmd.Flags().StringVar(&searchContract, "contract", "", "Filter by contract ID")
	searchCmd.Flags().BoolVar(&searchRegex, "regex", false, "Enable regex matching")

	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(searchCmd)
}
