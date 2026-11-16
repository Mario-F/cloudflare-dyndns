package providers

import (
	"io"

	"github.com/Mario-F/cloudflare-dyndns/internal/logger"
	"github.com/Mario-F/cloudflare-dyndns/internal/network"
)

func ipify6GetIP() (string, error) {
	logger.Debugf("Start GetIP with ipify6")

	resp, err := httpClient.Get("https://api6.ipify.org")
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
	logger.Debugf("Found IP with ipify6: %+v\n", ip)
	return ip, nil
}

var ipify6Provider Provider = Provider{
	GetIP:        ipify6GetIP,
	Version:      network.IPv6,
	ProviderName: "ipify6",
}

func init() {
	ProviderList = append(ProviderList, ipify6Provider)
}
