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
	"github.com/unsample/unsample/internal/cli"
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

	// Debug command
	var (
		cfgPath string
		curlCmd string
	)

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
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig(cfgPath)
			if err != nil {
				return err
			}

			opts := cli.DefaultDebugOpts()
			opts.CurlCmd = curlCmd

			// URL is required unless --curl is provided.
			var rawURL string
			if curlCmd != "" {
				rawURL = "" // URL comes from the curl command
			} else {
				if len(args) == 0 {
					return fmt.Errorf("URL is required\n\nUsage: unsample debug <url>\n   or: unsample debug --curl '<curl command>'")
				}
				rawURL = args[0]
			}

			return cli.RunDebug(cmd.Context(), cfg, rawURL, opts)
		},
	}

	debugCmd.Flags().StringVar(&curlCmd, "curl", "", "Parse and send a curl command string")
	debugCmd.Flags().StringVarP(&cfgPath, "config", "c", "", "Path to config file (default: ~/.unsample/config.yaml)")

	rootCmd.AddCommand(debugCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
