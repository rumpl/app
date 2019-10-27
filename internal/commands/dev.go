package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/docker/app/internal/packager"
	"github.com/docker/app/render"
	"github.com/docker/cli/cli"
	"github.com/pkg/errors"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/stack"
	"github.com/docker/cli/cli/command/stack/options"
	"github.com/docker/cli/cli/command/stack/swarm"
	composetypes "github.com/docker/cli/cli/compose/types"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
			app, err := packager.Extract(application)
			if err != nil {
				return errors.Wrap(err, "extract")
			}
			defer app.Cleanup()

			parameters := packager.ExtractCNABParametersValues(packager.ExtractCNABParameterMapping(app.Parameters()), os.Environ())
			rendered, err := render.Render(app, parameters, nil)
			if err != nil {
				return err
			}
			services := []composetypes.ServiceConfig{}
			on := composetypes.ServiceConfig{}
			for _, service := range rendered.Services {
				if opts.on[0] != service.Name {
					services = append(services, service)
				} else {
					on = service
					fmt.Printf("Not adding %q\n", service.Name)
				}
			}
			rendered.Services = services
			orchestrator, err := dockerCli.StackOrchestrator("kubernetes")
			if err != nil {
				return err
			}

			err = stack.RunDeploy(dockerCli,
				getFlagset(),
				rendered,
				orchestrator,
				options.Deploy{
					Namespace:        application,
					ResolveImage:     swarm.ResolveImageAlways,
					SendRegistryAuth: false,
				})
			if err != nil {
				return err
			}
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			b := exec.Command("docker", "build", "-t", on.Name, on.Name)
			err = b.Run()
			if err != nil {
				return err
			}
			// fmt.Println("telepresence",
			// 	"--new-deployment", on.Name,
			// 	"--expose", fmt.Sprint(on.Ports[0].Published)+":"+fmt.Sprint(on.Ports[0].Target),
			// 	"--docker-run", "--rm",
			// 	"-p", fmt.Sprint(on.Ports[0].Published)+":"+fmt.Sprint(on.Ports[0].Target),
			// 	"-v", dir+"/vote:/app",
			// 	on.Name)
			tele := exec.Command("telepresence",
				"--new-deployment", on.Name,
				"--expose", fmt.Sprint(on.Ports[0].Published)+":"+fmt.Sprint(on.Ports[0].Target),
				"--docker-run", "--rm",
				"-p", fmt.Sprint(on.Ports[0].Published)+":"+fmt.Sprint(on.Ports[0].Target),
				"-v", dir+"/vote:/app",
				on.Name)

			return tele.Run()
		},
	}
	cmd.Flags().StringArrayVar(&opts.on, "on", []string{}, "The service you are working on")
	cmd.Flags().StringArrayVar(&opts.debug, "debug", []string{}, "The service you want to debug")
	return cmd
}

func getFlagset() *pflag.FlagSet {
	result := pflag.NewFlagSet("", pflag.ContinueOnError)
	result.String("namespace", "default", "")
	return result
}
