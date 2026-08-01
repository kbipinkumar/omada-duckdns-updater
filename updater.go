package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type UpdateState struct {
	sync.RWMutex
	LastRunTime time.Time
	LastIPv4    string
	LastIPv6    string
	LastIPv4UpdateTime time.Time
	LastIPv6UpdateTime time.Time
	LastStatus  string
	LastError   string
	LastWarning string
}

var globalState UpdateState

func isPublicIP(ipStr string) (bool, string) {
	if ipStr == "" {
		return false, "empty IP"
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, "invalid IP format"
	}
	if ip.IsLoopback() {
		return false, "loopback address"
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false, "link-local address"
	}
	if ip.IsPrivate() {
		return false, "private address"
	}
	// Check CG-NAT (100.64.0.0/10)
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && (ip4[1]&192) == 64 {
			return false, "CG-NAT address"
		}
	}
	return true, ""
}

func createHTTPClient() *http.Client {
	// Skip TLS verify for Omada local connections
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: tr}
}

func getOmadaToken(config *Config) (string, error) {
	client := createHTTPClient()
	u := fmt.Sprintf("%s/openapi/authorize/token?grant_type=client_credentials", config.OmadaURL)

	payload := map[string]string{
		"omadacId":      config.OmadaOmadacID,
		"client_id":     config.OmadaClientID,
		"client_secret": config.OmadaClientSecret,
	}
	
	body, _ := json.Marshal(payload)
	resp, err := client.Post(u, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var data struct {
		ErrorCode int `json:"errorCode"`
		Result    struct {
			AccessToken string `json:"accessToken"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	if data.ErrorCode != 0 {
		return "", fmt.Errorf("Omada error code: %d", data.ErrorCode)
	}

	return data.Result.AccessToken, nil
}

func getGatewayWAN(config *Config, token string) (ipv4, ipv6 string, err error) {
	client := createHTTPClient()

	// 1. Get devices to find Gateway MAC
	devicesURL := fmt.Sprintf("%s/openapi/v1/%s/sites/%s/devices?page=1&pageSize=10", config.OmadaURL, config.OmadaOmadacID, config.OmadaSiteID)
	req, _ := http.NewRequest("GET", devicesURL, nil)
	req.Header.Set("Authorization", "AccessToken="+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var devicesData struct {
		ErrorCode int `json:"errorCode"`
		Result    struct {
			Data []struct {
				Type string `json:"type"`
				Mac  string `json:"mac"`
			} `json:"data"`
		} `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&devicesData); err != nil {
		return "", "", err
	}

	var gatewayMac string
	for _, dev := range devicesData.Result.Data {
		if dev.Type == "gateway" {
			gatewayMac = dev.Mac
			break
		}
	}

	if gatewayMac == "" {
		return "", "", fmt.Errorf("gateway not found")
	}

	// 2. Get WAN status
	wanURL := fmt.Sprintf("%s/openapi/v1/%s/sites/%s/gateways/%s/wan-status", config.OmadaURL, config.OmadaOmadacID, config.OmadaSiteID, gatewayMac)
	req, _ = http.NewRequest("GET", wanURL, nil)
	req.Header.Set("Authorization", "AccessToken="+token)

	resp2, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp2.Body.Close()

	var wanData struct {
		ErrorCode int `json:"errorCode"`
		Result    []struct {
			Name       string `json:"name"`
			Port       int    `json:"port"`
			IP         string `json:"ip"`
			LinkStatus int    `json:"linkStatus"`
			WanPortIpv6Config struct {
				Addr string `json:"addr"`
			} `json:"wanPortIpv6Config"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp2.Body).Decode(&wanData); err != nil {
		return "", "", err
	}

	for _, port := range wanData.Result {
		nameUpper := strings.ToUpper(port.Name)
		if nameUpper == "WAN" || nameUpper == "WAN1" || nameUpper == "WAN/LAN1" || port.Port == 1 {
			ipv4Empty := (port.IP == "" || port.IP == "0.0.0.0")
			ipv6Empty := (port.WanPortIpv6Config.Addr == "" || port.WanPortIpv6Config.Addr == "::")
			if ipv4Empty && ipv6Empty {
				return "", "", fmt.Errorf("WAN_DOWN")
			}
			return port.IP, port.WanPortIpv6Config.Addr, nil
		}
	}

	return "", "", nil
}

func updateDuckDNS(config *Config, ipv4, ipv6 string) error {
	params := url.Values{}
	params.Add("domains", config.DuckDNSDomain)
	params.Add("token", config.DuckDNSToken)
	if ipv4 != "" {
		params.Add("ip", ipv4)
	}
	if ipv6 != "" {
		params.Add("ipv6", ipv6)
	}

	u := fmt.Sprintf("https://www.duckdns.org/update?%s", params.Encode())
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(bodyBytes))

	if bodyStr != "OK" {
		return fmt.Errorf("duckdns error: %s", bodyStr)
	}
	return nil
}

type Site struct {
	Id   string `json:"siteId"`
	Name string `json:"name"`
}

func fetchSites(config *Config, token string) ([]Site, error) {
	client := createHTTPClient()
	u := fmt.Sprintf("%s/openapi/v1/%s/sites?page=1&pageSize=100", config.OmadaURL, config.OmadaOmadacID)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "AccessToken="+token)
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var data struct {
		ErrorCode int `json:"errorCode"`
		Result    struct {
			Data []Site `json:"data"`
		} `json:"result"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if data.ErrorCode != 0 {
		return nil, fmt.Errorf("omada error code: %d", data.ErrorCode)
	}
	
	return data.Result.Data, nil
}

func runUpdate(force bool) error {
	globalState.Lock()
	defer globalState.Unlock()
	globalState.LastRunTime = time.Now()
	globalState.LastWarning = ""

	config, err := loadConfig()
	if err != nil {
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to load config: " + err.Error()
		return fmt.Errorf("failed to load config: %v", err)
	}

	if config.OmadaURL == "" || config.DuckDNSToken == "" || config.OmadaSiteID == "" {
		globalState.LastStatus = "Error"
		globalState.LastError = "Configuration is missing required fields (Check Site ID)"
		return fmt.Errorf("configuration is missing required fields")
	}

	token, err := getOmadaToken(config)
	if err != nil {
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to get omada token: " + err.Error()
		return fmt.Errorf("failed to get omada token: %v", err)
	}

	ipv4, ipv6, err := getGatewayWAN(config, token)
	if err != nil {
		if err.Error() == "WAN_DOWN" {
			globalState.LastStatus = "Skipped (WAN Down)"
			globalState.LastError = ""
			globalState.LastWarning = "WAN interface is down or has no IP from ISP."
			return nil
		}
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to get gateway WAN: " + err.Error()
		return fmt.Errorf("failed to get gateway WAN: %v", err)
	}
	
	ipv4Changed := ipv4 != "" && ipv4 != globalState.LastIPv4
	ipv6Changed := ipv6 != "" && ipv6 != globalState.LastIPv6
	
	globalState.LastIPv4 = ipv4
	globalState.LastIPv6 = ipv6

	var warnings []string

	if ipv4 != "" {
		if ok, reason := isPublicIP(ipv4); !ok {
			warnings = append(warnings, fmt.Sprintf("IPv4 ignored (%s)", reason))
			ipv4 = ""
			ipv4Changed = false
		}
	}

	if ipv6 != "" {
		if ok, reason := isPublicIP(ipv6); !ok {
			warnings = append(warnings, fmt.Sprintf("IPv6 ignored (%s)", reason))
			ipv6 = ""
			ipv6Changed = false
		}
	}

	if ipv4 == "" && ipv6 == "" {
		globalState.LastStatus = "Error"
		if len(warnings) > 0 {
			globalState.LastError = "No valid public IPs: " + strings.Join(warnings, ", ")
		} else {
			globalState.LastError = "No IPs found on gateway"
		}
		return fmt.Errorf("no usable public IPs found")
	}

	if !force && !ipv4Changed && !ipv6Changed {
		globalState.LastStatus = "Skipped (No IP Change)"
		globalState.LastError = ""
		if len(warnings) > 0 {
			globalState.LastWarning = strings.Join(warnings, " | ")
		} else {
			globalState.LastWarning = ""
		}
		return nil
	}

	err = updateDuckDNS(config, ipv4, ipv6)
	if err != nil {
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to update duckdns: " + err.Error()
		return fmt.Errorf("failed to update duckdns: %v", err)
	}

	now := time.Now()
	if ipv4 != "" {
		globalState.LastIPv4UpdateTime = now
	}
	if ipv6 != "" {
		globalState.LastIPv6UpdateTime = now
	}

	globalState.LastError = ""
	if len(warnings) > 0 {
		globalState.LastStatus = "Success (with warnings)"
		globalState.LastWarning = strings.Join(warnings, " | ")
	} else {
		globalState.LastStatus = "Success (OK)"
	}
	return nil
}
