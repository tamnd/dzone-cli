package cli

import (
	"github.com/spf13/cobra"
)

// latestCmd returns the `latest` command.
func (a *App) latestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "latest",
		Short: "Fetch the latest DZone articles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching latest articles...")
			arts, err := a.client.Latest(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(arts, len(arts))
		},
	}
}
