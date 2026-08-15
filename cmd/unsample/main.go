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

	// Init command
	var (
		initDir   string
		initForce bool
	)

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Generate Collector config + setup guide",
		Long: `Generate the configuration files needed to run Unsample locally.

Creates a .unsample/ directory with:
  - config.yaml          CLI config with a generated secret
  - docker-compose.yaml  OTel Collector + Tempo + Grafana
  - otel-collector-config.yaml
  - tempo-config.yaml
  - grafana-datasources.yaml

Run 'docker compose -f .unsample/docker-compose.yaml up -d' to start.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.RunInit(cli.InitOpts{
				Dir:   initDir,
				Force: initForce,
			})
		},
	}

	initCmd.Flags().StringVar(&initDir, "dir", ".unsample", "Output directory for generated files")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing files")

	rootCmd.AddCommand(initCmd)

	// Completion command (built-in from Cobra)
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for unsample.

To load completions:

  bash:
    source <(unsample completion bash)

  zsh:
    echo 'source <(unsample completion zsh)' >> ~/.zshrc

  fish:
    unsample completion fish | source

  powershell:
    unsample completion powershell | Out-String | Invoke-Expression`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				return rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				return rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}

	rootCmd.AddCommand(completionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
