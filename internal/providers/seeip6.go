package providers

import (
	"io"

	"github.com/Mario-F/cloudflare-dyndns/internal/logger"
	"github.com/Mario-F/cloudflare-dyndns/internal/network"
)

func seeip6GetIP() (string, error) {
	logger.Debugf("Start GetIP with seeip6")

	resp, err := httpClient.Get("https://ipv6.seeip.org")
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

	ip, err := captureIPv6(string(body))
	if err != nil {
		return "", err
	}
	if ip == "" {
		return "", errIPNotFound
	}
	logger.Debugf("Found IP with seeip6: %+v\n", ip)
	return ip, nil
}

var seeip6Provider Provider = Provider{
	GetIP:        seeip6GetIP,
	Version:      network.IPv6,
	ProviderName: "seeip6",
}

func init() {
	ProviderList = append(ProviderList, seeip6Provider)
}
