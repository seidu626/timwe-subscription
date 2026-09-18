package main

import "testing"

func TestConfigValidation(t *testing.T) {
	for _, edit := range []func(*Config){
		func(c *Config) { c.Source = "" },
		func(c *Config) { c.Source = "database" },
		func(c *Config) { c.TenantKey = "" },
		func(c *Config) { c.ChannelKey = "" },
		func(c *Config) { c.ProductIDs = nil },
		func(c *Config) { c.BatchSize = 0 },
		func(c *Config) { c.MaxSourceBytes = 0 },
		func(c *Config) { c.PollInterval = "0s" },
		func(c *Config) { c.MaxPollingDuration = "-1s" },
		func(c *Config) { c.WaitBetweenCalls = "bad" },
		func(c *Config) { c.BaseURL = "http://user:password@localhost" },
	} {
		c := testConfig(t)
		edit(&c)
		if err := c.validate(); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
