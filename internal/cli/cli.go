package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var startTUI func(file, env string)

func SetTUIStarter(fn func(file, env string)) {
	startTUI = fn
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var envFile string

	cmd := &cobra.Command{
		Use:   "lazyapi [file]",
		Short: "OpenAPI-driven API exploration, testing, and automation from the terminal",
		Long: `OpenAPI-driven API exploration, testing, and automation from the terminal.

TUI (interactive):
  lazyapi                   Start the terminal UI with no spec
  lazyapi <file.yml>        Start the TUI with an OpenAPI spec preloaded
  lazyapi <file.yml> --env <file>  Start the TUI with an env file

CLI (headless commands):
  create [name] [servers...]          Create an OpenAPI template
  rm <file> <method> <path>           Remove a request from a spec
  add request <file> <path> <method>  Add a request to a spec
  add server <file> <url>             Add a server URL to a spec
  send <file> <path> <method>         Send a request from the spec
    --server <url|index>              Server URL (or index into spec servers)
    --env <file>                      Environment file for variable substitution
    --save-example                    Save response as example in the spec
  smoke <file>                        Run smoke tests against the API
    --server <url>                    Base server URL
    --env <file>                      Environment file`,
		Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if startTUI == nil {
				return nil
			}
			file := ""
			if len(args) > 0 {
				file = args[0]
			}
			startTUI(file, envFile)
			return nil
		},
	}

	cmd.Flags().StringVar(&envFile, "env", "", "Environment file for variable substitution")
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newSendCmd())
	cmd.AddCommand(newSmokeCmd())
	return cmd
}
