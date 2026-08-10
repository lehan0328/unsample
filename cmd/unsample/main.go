// Unsample — On-demand debug tracing for OpenTelemetry.
//
// Force full trace capture for any single request, regardless of your
// sampling configuration. Works with your existing OTel setup.
//
// Usage:
//
//	unsample debug <url>          Send a debug-traced request
//	unsample debug --curl '<cmd>' Parse and send a curl command
//	unsample init                 Generate Collector config + setup guide
//	unsample version              Print version
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/unsample/unsample/internal/version"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "unsample",
		Short: "On-demand debug tracing for OpenTelemetry",
		Long: `Unsample forces full distributed trace capture for any single request,
regardless of your sampling configuration.

When you're debugging a production issue and the trace was sampled away,
unsample ensures the next request you send is fully captured.

Works with your existing OTel-instrumented services and Collector.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version of unsample",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("unsample %s\n", version.Version)
		},
	})

	// Debug command (placeholder — will be fully implemented in Day 2)
	debugCmd := &cobra.Command{
		Use:   "debug <url>",
		Short: "Send a debug-traced HTTP request",
		Long: `Send an HTTP request with a debug trace token injected.
The request will be fully traced regardless of your sampling configuration.

The CLI will:
  1. Generate a signed debug token
  2. Inject it into the request headers (W3C baggage)
  3. Send the request and display the HTTP response
  4. Wait for the trace to be indexed
  5. Output a deep link to the trace viewer`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			fmt.Printf("🔍 Debug tracing: %s\n", url)
			fmt.Println("⚠️  CLI debug command will be fully implemented in Day 2")
			return nil
		},
	}

	// Debug command flags (will be wired in Day 2)
	debugCmd.Flags().String("curl", "", "Parse and send a curl command string")
	debugCmd.Flags().StringP("config", "c", "", "Path to config file (default: ~/.unsample/config.yaml)")

	rootCmd.AddCommand(debugCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
