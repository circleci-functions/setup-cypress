// Command setup-cypress installs and caches the Cypress binary, and runs a
// Cypress suite through CircleCI's testsuite, from a CircleCI job.
//
// Supported platforms: linux, darwin, windows.
package main

import (
	"context"

	"github.com/alecthomas/kong"

	"github.com/circleci-functions/toolkit/functionyaml"
	"github.com/circleci-functions/toolkit/sdk"
)

type cli struct {
	Run     runCmd     `cmd:"" default:"withargs" help:"Install the Cypress binary, then run a Cypress suite with circleci testsuite. Runs when no command is given."`
	Install installCmd `cmd:"" help:"Install the Cypress binary the project's Cypress package needs, caching it across jobs."`
}

func main() {
	sdk.Run(run)
}

func run(ctx context.Context, client *sdk.Client) error {
	var flags cli
	kCtx := kong.Parse(&flags,
		kong.Name("setup-cypress"),
		kong.Description("Install and cache Cypress, and run a Cypress suite through CircleCI's testsuite."),
		kong.UsageOnError(),
		functionyaml.Embed(),
	)
	kCtx.BindTo(ctx, (*context.Context)(nil))
	kCtx.Bind(client)
	return kCtx.Run()
}
