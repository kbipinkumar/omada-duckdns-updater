package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	OmadaURL          string
	OmadaClientID     string
	OmadaClientSecret string
	OmadaOmadacID     string
	OmadaSiteID       string
	DuckDNSToken      string
	DuckDNSDomain     string
	UpdateIPv4        bool
	UpdateIPv6        bool
	UpdateInterval    int
	WebUsername       string
	WebPassword       string
}

const configFilePath = "updater.conf"

func loadConfig() (*Config, error) {
	// Default to updating IPv4 only
	config := &Config{UpdateInterval: 5, UpdateIPv4: true, UpdateIPv6: false}
	file, err := os.Open(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "OMADA_URL":
			config.OmadaURL = val
		case "OMADA_CLIENT_ID":
			config.OmadaClientID = val
		case "OMADA_CLIENT_SECRET":
			config.OmadaClientSecret = deobfuscateToken(val)
		case "OMADA_OMADAC_ID":
			config.OmadaOmadacID = val
		case "OMADA_SITE_ID":
			config.OmadaSiteID = val
		case "DUCKDNS_TOKEN":
			config.DuckDNSToken = deobfuscateToken(val)
		case "DUCKDNS_DOMAIN":
			config.DuckDNSDomain = val
		case "UPDATE_IPV4":
			if b, err := strconv.ParseBool(val); err == nil {
				config.UpdateIPv4 = b
			}
		case "UPDATE_IPV6":
			if b, err := strconv.ParseBool(val); err == nil {
				config.UpdateIPv6 = b
			}
		case "UPDATE_INTERVAL":
			if i, err := strconv.Atoi(val); err == nil && i > 0 {
				config.UpdateInterval = i
			}
		case "WEB_USERNAME":
			config.WebUsername = val
		case "WEB_PASSWORD":
			config.WebPassword = val
		}
	}
	return config, scanner.Err()
}

func saveConfig(config *Config) error {
	file, err := os.Create(configFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	fmt.Fprintf(writer, "OMADA_URL=%s\n", config.OmadaURL)
	fmt.Fprintf(writer, "OMADA_CLIENT_ID=%s\n", config.OmadaClientID)
	fmt.Fprintf(writer, "OMADA_CLIENT_SECRET=%s\n", obfuscateToken(config.OmadaClientSecret))
	fmt.Fprintf(writer, "OMADA_OMADAC_ID=%s\n", config.OmadaOmadacID)
	fmt.Fprintf(writer, "OMADA_SITE_ID=%s\n", config.OmadaSiteID)
	fmt.Fprintf(writer, "DUCKDNS_TOKEN=%s\n", obfuscateToken(config.DuckDNSToken))
	fmt.Fprintf(writer, "DUCKDNS_DOMAIN=%s\n", config.DuckDNSDomain)
	fmt.Fprintf(writer, "UPDATE_IPV4=%t\n", config.UpdateIPv4)
	fmt.Fprintf(writer, "UPDATE_IPV6=%t\n", config.UpdateIPv6)
	if config.UpdateInterval <= 0 {
		config.UpdateInterval = 5
	}
	fmt.Fprintf(writer, "UPDATE_INTERVAL=%d\n", config.UpdateInterval)
	fmt.Fprintf(writer, "WEB_USERNAME=%s\n", config.WebUsername)
	fmt.Fprintf(writer, "WEB_PASSWORD=%s\n", config.WebPassword)
	return writer.Flush()
}

func (c *Config) DuckDNSDomains() []string {
	parts := strings.Split(c.DuckDNSDomain, ",")
	res := make([]string, 5)
	for i := 0; i < 5; i++ {
		if i < len(parts) {
			res[i] = strings.TrimSpace(parts[i])
		} else {
			res[i] = ""
		}
	}
	return res
}
