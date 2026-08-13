package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Config holds Omada, DuckDNS, scheduling, and web UI settings loaded from updater.conf.
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

// getDataDir returns the directory used for updater.conf (and related files).
// DATA_DIR overrides the default. On Windows the default is
// %ProgramData%\omada-duckdns-updater; elsewhere it is the current working directory.
func getDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return filepath.Join(programData, "omada-duckdns-updater")
	}
	return ""
}

// getConfigFilePath returns the path to updater.conf in the data directory.
func getConfigFilePath() string {
	dir := getDataDir()
	if dir != "" {
		return filepath.Join(dir, "updater.conf")
	}
	return "updater.conf"
}

// configFileExists reports whether updater.conf is present on disk.
func configFileExists() bool {
	_, err := os.Stat(getConfigFilePath())
	return err == nil
}

// isFirstRun reports whether the application has not yet been configured.
func isFirstRun(config *Config) bool {
	return !configFileExists() || config.OmadaURL == ""
}

// configIsComplete reports whether all fields required for an update cycle are set.
func configIsComplete(config *Config) bool {
	return len(missingConfigFields(config)) == 0
}

// missingConfigFields returns human-readable names of required config fields that are unset.
func missingConfigFields(config *Config) []string {
	if config == nil {
		return []string{"configuration"}
	}
	var missing []string
	if config.OmadaURL == "" {
		missing = append(missing, "Omada Controller URL")
	}
	if config.OmadaSiteID == "" {
		missing = append(missing, "Omada Site ID")
	}
	if config.DuckDNSToken == "" {
		missing = append(missing, "DuckDNS Token")
	}
	if config.DuckDNSDomain == "" {
		missing = append(missing, "DuckDNS Domain")
	}
	return missing
}

// ensureDataDir creates the data directory when one is configured.
func ensureDataDir() error {
	dir := getDataDir()
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// loadConfig reads updater.conf and returns defaults when the file is missing.
func loadConfig() (*Config, error) {
	// Default to updating IPv4 only
	config := &Config{UpdateInterval: 5, UpdateIPv4: true, UpdateIPv6: false}
	if err := ensureDataDir(); err != nil {
		return nil, err
	}
	file, err := os.Open(getConfigFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}
	defer file.Close()

	needsMigration := false
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
			plain, usedLegacy := decryptToken(val)
			config.OmadaClientSecret = plain
			if usedLegacy || (!strings.HasPrefix(val, "ENC:") && val != "") {
				needsMigration = true
			}
		case "OMADA_OMADAC_ID":
			config.OmadaOmadacID = val
		case "OMADA_SITE_ID":
			config.OmadaSiteID = val
		case "DUCKDNS_TOKEN":
			plain, usedLegacy := decryptToken(val)
			config.DuckDNSToken = plain
			if usedLegacy || (!strings.HasPrefix(val, "ENC:") && val != "") {
				needsMigration = true
			}
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
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	applyEnvOverrides(config)

	if needsMigration {
		logInfo("migrating configuration secrets to current encryption format (%s)", getConfigFilePath())
		if err := saveConfig(config); err != nil {
			logError("configuration migration failed: %v", err)
			return config, err
		}
	}
	return config, nil
}

// applyEnvOverrides replaces API token secrets with environment variables when set.
func applyEnvOverrides(config *Config) {
	if v := os.Getenv("OMADA_CLIENT_SECRET"); v != "" {
		config.OmadaClientSecret = v
	}
	if v := os.Getenv("DUCKDNS_TOKEN"); v != "" {
		config.DuckDNSToken = v
	}
}

// webPasswordForAuth returns the password used for Basic Auth checks.
func webPasswordForAuth(config *Config) string {
	if v := os.Getenv("WEB_PASSWORD"); v != "" {
		return v
	}
	return config.WebPassword
}

// verifyWebPassword checks the supplied password against stored or env-provided credentials.
func verifyWebPassword(password, stored string) bool {
	if v := os.Getenv("WEB_PASSWORD"); v != "" {
		return password == v
	}
	return checkPassword(password, stored)
}

// saveConfig writes config to updater.conf, obfuscating sensitive token values.
func saveConfig(config *Config) error {
	if err := ensureDataDir(); err != nil {
		return err
	}

	encryptedClientSecret := obfuscateToken(config.OmadaClientSecret)
	if encryptedClientSecret == config.OmadaClientSecret && config.OmadaClientSecret != "" && !strings.HasPrefix(config.OmadaClientSecret, "ENC:") {
		return fmt.Errorf("encryption unavailable: failed to encrypt OMADA_CLIENT_SECRET")
	}

	encryptedDuckDNSToken := obfuscateToken(config.DuckDNSToken)
	if encryptedDuckDNSToken == config.DuckDNSToken && config.DuckDNSToken != "" && !strings.HasPrefix(config.DuckDNSToken, "ENC:") {
		return fmt.Errorf("encryption unavailable: failed to encrypt DUCKDNS_TOKEN")
	}

	configPath := getConfigFilePath()
	dir := filepath.Dir(configPath)
	if dir == "" || dir == "." {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = wd
	}

	tempFile, err := os.CreateTemp(dir, ".updater.conf.tmp*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		os.Remove(tempPath)
	}()

	if err := tempFile.Chmod(0600); err != nil {
		return err
	}

	writer := bufio.NewWriter(tempFile)
	fmt.Fprintf(writer, "OMADA_URL=%s\n", config.OmadaURL)
	fmt.Fprintf(writer, "OMADA_CLIENT_ID=%s\n", config.OmadaClientID)
	fmt.Fprintf(writer, "OMADA_CLIENT_SECRET=%s\n", encryptedClientSecret)
	fmt.Fprintf(writer, "OMADA_OMADAC_ID=%s\n", config.OmadaOmadacID)
	fmt.Fprintf(writer, "OMADA_SITE_ID=%s\n", config.OmadaSiteID)
	fmt.Fprintf(writer, "DUCKDNS_TOKEN=%s\n", encryptedDuckDNSToken)
	fmt.Fprintf(writer, "DUCKDNS_DOMAIN=%s\n", config.DuckDNSDomain)
	fmt.Fprintf(writer, "UPDATE_IPV4=%t\n", config.UpdateIPv4)
	fmt.Fprintf(writer, "UPDATE_IPV6=%t\n", config.UpdateIPv6)
	if config.UpdateInterval <= 0 {
		config.UpdateInterval = 5
	}
	fmt.Fprintf(writer, "UPDATE_INTERVAL=%d\n", config.UpdateInterval)
	fmt.Fprintf(writer, "WEB_USERNAME=%s\n", config.WebUsername)
	fmt.Fprintf(writer, "WEB_PASSWORD=%s\n", config.WebPassword)
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	if err := os.Rename(tempPath, configPath); err != nil {
		return err
	}

	logInfo("configuration saved to %s (%s)", configPath, describeConfig(config))
	return nil
}

// DuckDNSDomains returns up to five configured DuckDNS domain names for the UI.
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
