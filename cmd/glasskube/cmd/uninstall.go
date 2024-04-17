package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	clientadapter "github.com/glasskube/glasskube/internal/adapter/goclient"
	"github.com/glasskube/glasskube/internal/cliutils"
	"github.com/glasskube/glasskube/internal/dependency"
	pkgClient "github.com/glasskube/glasskube/pkg/client"
	"github.com/glasskube/glasskube/pkg/statuswriter"
	"github.com/glasskube/glasskube/pkg/uninstall"
	"github.com/spf13/cobra"
)

var uninstallCmdOptions = struct {
	ForceUninstall bool
	NoWait         bool
}{}

var uninstallCmd = &cobra.Command{
	Use:    "uninstall [package-name]",
	Short:  "Uninstall a package",
	Long:   `Uninstall a package.`,
	Args:   cobra.ExactArgs(1),
	PreRun: cliutils.SetupClientContext(true, &rootCmdOptions.SkipUpdateCheck),
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]
		ctx := cmd.Context()
		currentContext := pkgClient.RawConfigFromContext(ctx).CurrentContext
		client := pkgClient.FromContext(ctx)
		dm := dependency.NewDependencyManager(clientadapter.NewPackageClientAdapter(client))

		if g, err := dm.NewGraph(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Error validating uninstall: %v\n", err)
			cliutils.ExitWithError()
		} else {
			g.Delete(pkgName)
			pruned := g.Prune()
			if err := g.Validate(); err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v can not be uninstalled for the following reason: %v\n", pkgName, err)
				cliutils.ExitWithError()
			} else {
				showUninstallDetails(currentContext, pkgName, pruned)
				if !uninstallCmdOptions.ForceUninstall && !cliutils.YesNoPrompt("Do you want to continue?", false) {
					fmt.Println("❌ Uninstallation cancelled.")
					cliutils.ExitSuccess()
				}
			}
		}

		uninstaller := uninstall.NewUninstaller(client).WithStatusWriter(statuswriter.Spinner())
		pkg := pkgClient.NewPackage(pkgName, "")
		if uninstallCmdOptions.NoWait {
			if err := uninstaller.Uninstall(ctx, pkg); err != nil {
				fmt.Fprintf(os.Stderr, "\n❌ An error occurred during uninstallation:\n\n%v\n", err)
				cliutils.ExitWithError()
			}
			fmt.Fprintln(os.Stderr, "Uninstallation started in background")
		} else {
			if err := uninstaller.UninstallBlocking(ctx, pkg); err != nil {
				fmt.Fprintf(os.Stderr, "\n❌ An error occurred during uninstallation:\n\n%v\n", err)
				cliutils.ExitWithError()
			}
			fmt.Fprintf(os.Stderr, "🗑️  %v uninstalled successfully.\n", pkgName)
		}
	},
}

func showUninstallDetails(context, name string, pruned []string) {
	fmt.Fprintf(os.Stderr,
		"The following packages will be %v from your cluster (%v):\n",
		color.New(color.Bold).Sprint("removed"),
		context)
	fmt.Fprintf(os.Stderr, " * %v (requested by user)\n", name)
	for _, dep := range pruned {
		fmt.Fprintf(os.Stderr, " * %v (dependency)\n", dep)
	}
}

func init() {
	uninstallCmd.PersistentFlags().BoolVar(&uninstallCmdOptions.ForceUninstall, "force", false,
		"skip the confirmation question and uninstall right away")
	uninstallCmd.PersistentFlags().BoolVar(&uninstallCmdOptions.NoWait, "no-wait", false, "perform non-blocking uninstall")
	RootCmd.AddCommand(uninstallCmd)
}
