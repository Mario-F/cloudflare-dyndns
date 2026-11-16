package providers

import (
	"io"

	"github.com/Mario-F/cloudflare-dyndns/internal/logger"
	"github.com/Mario-F/cloudflare-dyndns/internal/network"
)

func whoismyispGetIP() (string, error) {
	logger.Debugf("Start GetIP with whoismyisp")

	resp, err := httpClient.Get("https://www.whoismyisp.org/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errResponseNotOK
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip, err := captureIPv4(string(body))
	if err != nil {
		return "", err
	}
	if ip == "" {
		return "", errIPNotFound
	}
	logger.Debugf("Found IP with whoismyisp: %+v\n", ip)
	return ip, nil
}

var whoismyispProvider Provider = Provider{
	GetIP:        whoismyispGetIP,
	Version:      network.IPv4,
	ProviderName: "whoismyisp",
}

func init() {
	ProviderList = append(ProviderList, whoismyispProvider)
}
