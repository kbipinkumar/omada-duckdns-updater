package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// UpdateState tracks the most recent updater run and its outcome.
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

// StateView is a lock-free snapshot of UpdateState for template rendering.
type StateView struct {
	LastRunTime        time.Time
	LastIPv4           string
	LastIPv6           string
	LastIPv4UpdateTime time.Time
	LastIPv6UpdateTime time.Time
	LastStatus         string
	LastError          string
	LastWarning        string
}

// globalState holds the shared updater status used by the web UI.
var globalState UpdateState

// ErrWANDown indicates the Omada gateway WAN interface has no usable IP addresses.
var ErrWANDown = errors.New("WAN_DOWN")

// snapshotState returns a copy of globalState safe for template rendering.
func snapshotState() StateView {
	globalState.RLock()
	defer globalState.RUnlock()
	return StateView{
		LastRunTime:        globalState.LastRunTime,
		LastIPv4:           globalState.LastIPv4,
		LastIPv6:           globalState.LastIPv6,
		LastIPv4UpdateTime: globalState.LastIPv4UpdateTime,
		LastIPv6UpdateTime: globalState.LastIPv6UpdateTime,
		LastStatus:         globalState.LastStatus,
		LastError:          globalState.LastError,
		LastWarning:        globalState.LastWarning,
	}
}

// isPublicIP reports whether ipStr is a routable public address and, when false,
// returns a human-readable rejection reason.
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

// omadaURLHostRe validates the Host field (hostname or IP with optional port).
// A regex match on parsed.Host is a barrier guard recognized by CodeQL
// go/request-forgery between operator-supplied OMADA_URL and net/http requests.
var omadaURLHostRe = regexp.MustCompile(
	`^(` +
		`([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(\.([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?))*` +
		`|([0-9]{1,3}(\.[0-9]{1,3}){3})` +
		`|(\[[0-9a-fA-F:.]+\])` +
		`)(:[0-9]{1,5})?$`,
)

// createHTTPClient returns an HTTP client configured for local Omada controllers.
func createHTTPClient() *http.Client {
	// Skip TLS verify for Omada local connections
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &http.Client{Transport: tr}
}

type validateURLOpts struct {
	requireHTTPS  bool
	blockLoopback bool
	allowlist     []string
}

// ValidateURLOption configures validateRequestURL behavior.
type ValidateURLOption func(*validateURLOpts)

// WithRequireHTTPS requires the URL scheme to be https when true.
func WithRequireHTTPS(v bool) ValidateURLOption {
	return func(o *validateURLOpts) {
		o.requireHTTPS = v
	}
}

// WithBlockLoopback rejects hosts that resolve to loopback or unspecified addresses.
func WithBlockLoopback(v bool) ValidateURLOption {
	return func(o *validateURLOpts) {
		o.blockLoopback = v
	}
}

// WithAllowlist restricts the URL host to the given hostnames or IPs.
func WithAllowlist(hosts []string) ValidateURLOption {
	return func(o *validateURLOpts) {
		o.allowlist = hosts
	}
}

// validateRequestURL parses and validates a request URL before outbound HTTP calls.
func validateRequestURL(raw string, opts ...ValidateURLOption) (*url.URL, error) {
	options := validateURLOpts{
		requireHTTPS:  true,
		blockLoopback: false,
	}
	for _, opt := range opts {
		opt(&options)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("URL must include scheme and host")
	}
	if u.User != nil {
		return nil, fmt.Errorf("URL must not contain user info")
	}
	if options.requireHTTPS && u.Scheme != "https" {
		return nil, fmt.Errorf("only https scheme allowed")
	}

	if !omadaURLHostRe.MatchString(u.Host) {
		return nil, fmt.Errorf("invalid host %q", u.Host)
	}

	host := u.Hostname()
	if len(options.allowlist) > 0 {
		matched := false
		for _, allowed := range options.allowlist {
			if strings.EqualFold(allowed, host) {
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("host %q not in allowlist", host)
		}
	}

	if options.blockLoopback {
		ips, err := net.LookupIP(host)
		if err == nil {
			for _, ip := range ips {
				if ip.IsLoopback() || ip.IsUnspecified() {
					return nil, fmt.Errorf("resolved host %s -> %s is not allowed", host, ip.String())
				}
			}
		}
	}

	return &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
	}, nil
}

// omadaBaseURL validates config.OmadaURL before Omada OpenAPI requests.
func omadaBaseURL(config *Config) (*url.URL, error) {
	return validateRequestURL(
		config.OmadaURL,
		WithBlockLoopback(true),
		WithAllowlist(config.AllowedOmadaHosts),
	)
}

// resolveOmadaURL joins a validated Omada base URL with a relative API path.
func resolveOmadaURL(base *url.URL, path string) (string, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid API path: %w", err)
	}
	return base.ResolveReference(ref).String(), nil
}

// getOmadaToken exchanges client credentials for an Omada OpenAPI access token.
func getOmadaToken(config *Config) (string, error) {
	base, err := omadaBaseURL(config)
	if err != nil {
		return "", fmt.Errorf("omada URL rejected: %w", err)
	}

	client := createHTTPClient()
	u, err := resolveOmadaURL(base, "/openapi/authorize/token?grant_type=client_credentials")
	if err != nil {
		return "", err
	}

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

// getGatewayWAN fetches the primary WAN IPv4 and IPv6 addresses from Omada.
func getGatewayWAN(config *Config, token string) (ipv4, ipv6 string, err error) {
	base, err := omadaBaseURL(config)
	if err != nil {
		return "", "", fmt.Errorf("omada URL rejected: %w", err)
	}

	client := createHTTPClient()

	// 1. Get devices to find Gateway MAC
	devicesURL, err := resolveOmadaURL(base, fmt.Sprintf("/openapi/v1/%s/sites/%s/devices?page=1&pageSize=10", config.OmadaOmadacID, config.OmadaSiteID))
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequest("GET", devicesURL, nil)
	if err != nil {
		return "", "", err
	}
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
	wanURL, err := resolveOmadaURL(base, fmt.Sprintf("/openapi/v1/%s/sites/%s/gateways/%s/wan-status", config.OmadaOmadacID, config.OmadaSiteID, gatewayMac))
	if err != nil {
		return "", "", err
	}
	req, err = http.NewRequest("GET", wanURL, nil)
	if err != nil {
		return "", "", err
	}
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
				return "", "", ErrWANDown
			}
			return port.IP, port.WanPortIpv6Config.Addr, nil
		}
	}

	return "", "", nil
}

// updateDuckDNS sends the current public IPs to DuckDNS for the configured domains.
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

// Site represents an Omada site returned by the OpenAPI sites endpoint.
type Site struct {
	// Id is the Omada site identifier.
	Id string `json:"siteId"`
	// Name is the Omada site display name.
	Name string `json:"name"`
}

// fetchSites retrieves available Omada sites for the configured controller.
func fetchSites(config *Config, token string) ([]Site, error) {
	base, err := omadaBaseURL(config)
	if err != nil {
		return nil, fmt.Errorf("omada URL rejected: %w", err)
	}

	client := createHTTPClient()
	u, err := resolveOmadaURL(base, fmt.Sprintf("/openapi/v1/%s/sites?page=1&pageSize=100", config.OmadaOmadacID))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
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

// runUpdate performs one DuckDNS update cycle and updates globalState.
// When force is false, unchanged public IPs are skipped.
func runUpdate(force bool) error {
	globalState.Lock()
	defer globalState.Unlock()
	globalState.LastRunTime = time.Now()
	globalState.LastWarning = ""

	logInfo("update cycle started (force=%t)", force)

	config, err := loadConfig()
	if err != nil {
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to load config: " + err.Error()
		logError("update failed to load configuration: %v", err)
		return fmt.Errorf("failed to load config: %v", err)
	}

	if missing := missingConfigFields(config); len(missing) > 0 {
		globalState.LastStatus = "Error"
		globalState.LastError = "Configuration is missing required fields: " + strings.Join(missing, ", ")
		logError("update aborted: missing required fields [%s] (%s)", strings.Join(missing, ", "), describeConfig(config))
		return fmt.Errorf("configuration is missing required fields: %s", strings.Join(missing, ", "))
	}

	token, err := getOmadaToken(config)
	if err != nil {
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to get omada token: " + err.Error()
		logError("update failed to authenticate with Omada (site_id=%q url=%s): %v", config.OmadaSiteID, sanitizeURLForLog(config.OmadaURL), err)
		return fmt.Errorf("failed to get omada token: %v", err)
	}

	ipv4, ipv6, err := getGatewayWAN(config, token)
	if err != nil {
		if errors.Is(err, ErrWANDown) {
			globalState.LastStatus = "Skipped (WAN Down)"
			globalState.LastError = ""
			globalState.LastWarning = "WAN interface is down or has no IP from ISP."
			logWarn("update skipped: WAN interface is down or has no IP (site_id=%q)", config.OmadaSiteID)
			return nil
		}
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to get gateway WAN: " + err.Error()
		logError("update failed to fetch gateway WAN (site_id=%q): %v", config.OmadaSiteID, err)
		return fmt.Errorf("failed to get gateway WAN: %v", err)
	}
	
	ipv4Changed := ipv4 != "" && ipv4 != globalState.LastIPv4
	ipv6Changed := ipv6 != "" && ipv6 != globalState.LastIPv6
	
	globalState.LastIPv4 = ipv4
	globalState.LastIPv6 = ipv6

	if !config.UpdateIPv4 {
		ipv4 = ""
		ipv4Changed = false
	}
	if !config.UpdateIPv6 {
		ipv6 = ""
		ipv6Changed = false
	}

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
		if !config.UpdateIPv4 && !config.UpdateIPv6 {
			globalState.LastStatus = "Skipped"
			globalState.LastError = ""
			globalState.LastWarning = "Both IPv4 and IPv6 updates are disabled in configuration."
			logInfo("update skipped: both IPv4 and IPv6 updates are disabled")
			return nil
		}
		globalState.LastStatus = "Error"
		if len(warnings) > 0 {
			globalState.LastError = "No valid public IPs: " + strings.Join(warnings, ", ")
		} else {
			globalState.LastError = "No IPs found on gateway"
		}
		logError("update failed: no usable public IPs (%s)", strings.Join(warnings, ", "))
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
		logInfo("update skipped: no public IP change (ipv4=%q ipv6=%q)", ipv4, ipv6)
		return nil
	}

	err = updateDuckDNS(config, ipv4, ipv6)
	if err != nil {
		globalState.LastStatus = "Error"
		globalState.LastError = "Failed to update duckdns: " + err.Error()
		logError("update failed to update DuckDNS (ipv4=%q ipv6=%q domains=%q): %v", ipv4, ipv6, config.DuckDNSDomain, err)
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
		logInfo("update finished with warnings (ipv4=%q ipv6=%q warnings=%q)", ipv4, ipv6, globalState.LastWarning)
	} else {
		globalState.LastStatus = "Success (OK)"
		logInfo("update finished successfully (ipv4=%q ipv6=%q)", ipv4, ipv6)
	}
	return nil
}
