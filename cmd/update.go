package cmd

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/Mario-F/cloudflare-dyndns/internal/cloudflare"
	"github.com/Mario-F/cloudflare-dyndns/internal/externalip"
	"github.com/Mario-F/cloudflare-dyndns/internal/logger"
	"github.com/Mario-F/cloudflare-dyndns/internal/network"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var recordInputs []string

type recordClient interface {
	Verify() error
	Record(ref, recordType string) (cloudflare.Record, error)
	UpdateContent(zoneID, recordID, content string) (cloudflare.Record, error)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update records with external IP",
	RunE: func(cmd *cobra.Command, args []string) error {
		refs := recordInputs
		if len(refs) == 0 {
			refs = viper.GetStringSlice("record")
		}
		if len(refs) == 0 {
			return errors.New("no record given, use --record (repeatable) or record in the config file")
		}
		t, err := apiToken()
		if err != nil {
			return err
		}
		getIP := func() (string, error) {
			res, err := externalip.GetExternalIP(network.IPVersion(ipVersion))
			return res.IP, err
		}
		return runUpdate(cloudflare.New(t), getIP, refs, recordType())
	},
}

func runUpdate(client recordClient, getIP func() (string, error), refs []string, recordType string) error {
	var (
		wg       sync.WaitGroup
		myip     string
		ipErr    error
		verifyEr error
		records  = make([]cloudflare.Record, len(refs))
		errs     = make([]error, len(refs))
	)

	logger.Infof("Requesting external IP and Cloudflare DNS records")
	wg.Add(2)
	go func() {
		defer wg.Done()
		myip, ipErr = getIP()
	}()
	go func() {
		defer wg.Done()
		if verifyEr = client.Verify(); verifyEr != nil {
			return
		}
		for i, ref := range refs {
			records[i], errs[i] = client.Record(ref, recordType)
		}
	}()
	wg.Wait()

	if ipErr != nil {
		return fmt.Errorf("get external IP: %w", ipErr)
	}
	if verifyEr != nil {
		return verifyEr
	}
	logger.Infof("New - IP: %s", myip)

	failed := 0
	for i, ref := range refs {
		if errs[i] != nil {
			logger.Errorf("%s: %v", ref, errs[i])
			failed++
			continue
		}
		r := records[i]
		logger.Infof("Domain: %s (%s)", r.Name, r.Ref())
		logger.Infof("Old - IP: %s", r.Content)
		if sameIP(r.Content, myip) {
			logger.Infof("No IP diff detected, update not necessary.")
			continue
		}
		if _, err := client.UpdateContent(r.ZoneID, r.ID, myip); err != nil {
			logger.Errorf("%s: update failed: %v", ref, err)
			failed++
			continue
		}
		logger.Infof("Record updated successfully")
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d records failed", failed, len(refs))
	}
	return nil
}

func sameIP(a, b string) bool {
	ipA, ipB := net.ParseIP(a), net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return a == b
	}
	return ipA.Equal(ipB)
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVar(&token, "token", "", "Cloudflare API token (prefer "+tokenEnv+")")
	updateCmd.Flags().StringArrayVar(&recordInputs, "record", nil, "Record to update, FQDN or <zone_id>/<record_id> (repeatable)")
}
