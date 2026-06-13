package cli

import (
	"github.com/spf13/cobra"
)

// feedCmd returns the `feed <section>` command.
func (a *App) feedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "feed <section>",
		Short: "Fetch articles from a named DZone topic feed",
		Long: `Fetch articles from a named DZone topic feed.

Available sections: java, security, microservices, performance, agile,
big-data, open-source, data, iot, languages, integration.

Run "dz sections" to list all sections with their URLs.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			section := args[0]
			n := a.effectiveLimit(20)
			a.progressf("fetching %s feed...", section)
			arts, err := a.client.Feed(cmd.Context(), section, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(arts, len(arts))
		},
	}
}

// sectionsCmd returns the `sections` command.
func (a *App) sectionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sections",
		Short: "List available DZone topic sections",
		RunE: func(cmd *cobra.Command, _ []string) error {
			secs := a.client.Sections()
			return a.renderOrEmpty(secs, len(secs))
		},
	}
}
