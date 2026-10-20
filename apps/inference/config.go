package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Model struct {
	Alias     string `json:"alias"`
	Revision  string `json:"revision"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	URL       string `json:"url"`
	Enabled   bool   `json:"enabled"`
	MaxInput  int64  `json:"max_input_tokens"`
	MaxOutput int64  `json:"max_output_tokens"`
	// Rates are USD micros per million tokens; conservative upper rates include
	// any long-context premium. No provider discounts or caching are assumed.
	InputRate       int64  `json:"input_micros_per_million"`
	OutputRate      int64  `json:"output_micros_per_million"`
	PriceVersion    string `json:"price_version"`
	PriceValidUntil string `json:"price_valid_until"`
}
type Config struct {
	LocalPaused     bool    `json:"local_paused"`
	Models          []Model `json:"models"`
	MonthlyMicros   int64   `json:"monthly_micros"`
	CloudEnabled    bool    `json:"cloud_enabled"`
	LocalConcurrent int     `json:"local_concurrent"`
	CloudConcurrent int     `json:"cloud_concurrent"`
}
type Client struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	Cloud bool   `json:"cloud"`
}

func loadConfig(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, e
	}
	return c, c.validate()
}
func (c Config) validate() error {
	if c.MonthlyMicros < 1 || c.MonthlyMicros > 100_000_000 || c.LocalConcurrent < 1 || c.LocalConcurrent > 2 || c.CloudConcurrent < 1 || c.CloudConcurrent > 2 {
		return fmt.Errorf("invalid budget or concurrency")
	}
	seen := map[string]bool{}
	for _, m := range c.Models {
		if seen[m.Alias] || m.Revision == "" || m.MaxInput < 1 || m.MaxInput > 1_000_000 || m.MaxOutput < 1 || m.MaxOutput > 4096 {
			return fmt.Errorf("invalid model profile")
		}
		seen[m.Alias] = true
		u, e := url.Parse(m.URL)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("invalid configured gateway URL")
		}
		if m.Provider == "local" {
			if m.Alias != "local-default" || m.Model == "" || m.InputRate != 0 || m.OutputRate != 0 {
				return fmt.Errorf("invalid local profile")
			}
		} else if m.Provider == "bedrock" {
			allowed := map[string]string{"bedrock-nova": "eu.amazon.nova-2-lite-v1:0", "bedrock-sonnet": "eu.anthropic.claude-sonnet-4-5-20250929-v1:0"}
			if allowed[m.Alias] != m.Model || m.Model == "" || m.MaxInput != 1_000_000 {
				return fmt.Errorf("unapproved Bedrock model")
			}
			// Expiry blocks dispatch, but must not prevent local startup after a restart.
			if c.CloudEnabled && m.Enabled && !m.priced(time.Time{}) {
				return fmt.Errorf("enabled cloud profile requires current verified pricing")
			}
		} else {
			return fmt.Errorf("invalid provider")
		}
	}
	if !seen["local-default"] {
		return fmt.Errorf("local-default profile required")
	}
	return nil
}
func (m Model) priced(now time.Time) bool {
	until, e := time.Parse(time.RFC3339, m.PriceValidUntil)
	return e == nil && now.Before(until) && m.PriceVersion != "" && m.InputRate > 0 && m.InputRate <= 1_000_000_000 && m.OutputRate > 0 && m.OutputRate <= 1_000_000_000
}
func (m Model) cost(input, output int64) int64 {
	return (input*m.InputRate+999999)/1_000_000 + (output*m.OutputRate+999999)/1_000_000
}
func validateClients(clients []Client) error {
	ids, tokens := map[string]bool{}, map[string]bool{}
	if len(clients) == 0 {
		return fmt.Errorf("client credentials required")
	}
	for _, c := range clients {
		if c.ID == "" || len(c.Token) < 32 || strings.TrimSpace(c.Token) != c.Token || ids[c.ID] || tokens[c.Token] {
			return fmt.Errorf("invalid or duplicate client credential")
		}
		ids[c.ID] = true
		tokens[c.Token] = true
	}
	return nil
}
