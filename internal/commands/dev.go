package commands

import (
	"fmt"
	"os"

	"github.com/deislabs/cnab-go/action"
	"github.com/docker/app/internal/cnab"
	"github.com/docker/app/internal/packager"
	"github.com/docker/app/internal/store"
	"github.com/docker/cli/cli"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/pkg/errors"

	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
)

type devOptions struct {
	on    []string
	debug []string
}

func devCmd(dockerCli command.Cli) *cobra.Command {
	var opts devOptions

	cmd := &cobra.Command{
		Use:     "dev [OPTIONS] APP",
		Aliases: []string{"deploy"},
		Short:   "develop a docker app",
		Args:    cli.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			application := args[0]
			composeApp, err := packager.Extract(application)
			if err != nil {
				return errors.Wrap(err, "extract")
			}
			defer composeApp.Cleanup()

			bundle, err := packager.MakeBundleFromApp(dockerCli, composeApp, nil)
			if err != nil {
				return errors.Wrap(err, "make bundle")
			}
			for name, im := range bundle.Images {
				fmt.Println(name, im.Image)
			}
			bind, err := cnab.RequiredBindMount("default", "kubernetes", dockerCli.ContextStore())
			if err != nil {
				return errors.Wrap(err, "requirebindmount")
			}
			driverImpl, errBuf := cnab.PrepareDriver(dockerCli, bind, nil)

			installation, err := store.NewInstallation(namesgenerator.GetRandomName(0), "")
			installation.Bundle = bundle
			inst := &action.Install{
				Driver: driverImpl,
			}
			// bundle.Credentials[internal.CredentialDockerContextName] = "default"
			creds, err := prepareCredentialSet(bundle)
			if err != nil {
				return errors.Wrap(err, "creds")
			}
			{
				defer muteDockerCli(dockerCli)()
				err = inst.Run(&installation.Claim, creds, os.Stdout)
				if err != nil {
					return fmt.Errorf("Failed to run App: %s\n%s", err, errBuf)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&opts.on, "on", []string{}, "The service you are working on")
	cmd.Flags().StringArrayVar(&opts.debug, "debug", []string{}, "The service you want to debug")
	return cmd
}
