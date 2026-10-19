package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/NSObjects/echo-admin/internal/boot"
)

type accessAuthorizationUpgrade func(context.Context, string) error

func newUpgradeAuthorizationCommand(upgrade accessAuthorizationUpgrade) *cobra.Command {
	var configPath string
	command := &cobra.Command{
		Use:   "upgrade-access-authorization",
		Short: "Reconcile the persisted access catalog with this deployment",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return upgrade(command.Context(), configPath)
		},
	}
	command.Flags().StringVar(&configPath, "config", "configs/config.toml", "config file")
	return command
}

func init() {
	rootCmd.AddCommand(newUpgradeAuthorizationCommand(boot.UpgradeAccessAuthorization))
}
