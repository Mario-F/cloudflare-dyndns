package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/Mario-F/cloudflare-dyndns/internal/cloudflare"
	"github.com/Mario-F/cloudflare-dyndns/internal/logger"
	"github.com/spf13/cobra"
)

var recordsCmd = &cobra.Command{
	Use:   "records",
	Short: "List all A (or AAAA) records of all zones the token can access",
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := apiToken()
		if err != nil {
			return err
		}
		client := cloudflare.New(t)
		if err := client.Verify(); err != nil {
			return err
		}
		zones, err := client.Zones()
		if err != nil {
			return err
		}
		logger.Debugf("Found %d zones", len(zones))

		var all []cloudflare.Record
		for _, z := range zones {
			records, err := client.Records(z, recordType(), "")
			if err != nil {
				return fmt.Errorf("zone %s: %w", z.Name, err)
			}
			all = append(all, records...)
		}
		if len(all) == 0 {
			fmt.Println("No records found")
			return nil
		}
		sort.SliceStable(all, func(i, j int) bool { return all[i].Name < all[j].Name })

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tFQDN\tVALUE\tTTL\tPROXIED")
		for _, r := range all {
			ttl := fmt.Sprint(r.TTL)
			if r.TTL == 1 {
				ttl = "auto"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\n", r.Ref(), r.Name, r.Content, ttl, r.Proxied)
		}
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(recordsCmd)

	recordsCmd.Flags().StringVar(&token, "token", "", "Cloudflare API token (prefer "+tokenEnv+")")
}
