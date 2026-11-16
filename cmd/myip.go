package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Mario-F/cloudflare-dyndns/internal/externalip"
	"github.com/Mario-F/cloudflare-dyndns/internal/network"
	"github.com/spf13/cobra"
)

var OutputModes = []string{
	"text",
	"short",
	"json",
}

var outputMode string

var myipCmd = &cobra.Command{
	Use:   "myip",
	Short: "Acquire your external IP and output it",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := externalip.GetExternalIP(network.IPVersion(ipVersion))
		if err != nil {
			return err
		}

		switch outputMode {
		case "text":
			fmt.Printf("Your external IP is: %s\n", res.IP)
		case "short":
			fmt.Printf("%s", res.IP)
		case "json":
			jsonRes, err := json.Marshal(res)
			if err != nil {
				return err
			}
			fmt.Printf("%s", jsonRes)
		default:
			return fmt.Errorf("output mode is not valid, use one of: %s", strings.Join(OutputModes, ", "))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(myipCmd)

	myipCmd.Flags().StringVarP(&outputMode, "output", "o", "text", fmt.Sprintf("How the result should be formatted.\nAllowed values: %s", strings.Join(OutputModes, ", ")))
}
