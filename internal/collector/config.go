package unsampleprocessor

import (
	"errors"
	"fmt"
)

// Config defines the configuration for the unsample processor.
type Config struct {
	// DebugAttribute is the span attribute key used to identify debug spans.
	// Spans with this attribute set to true are routed to the debug pipeline.
	DebugAttribute string `mapstructure:"debug_attribute"`

	// MaxPerMinute is the maximum number of debug spans allowed per minute.
	// Excess debug spans are silently dropped (never retried).
	MaxPerMinute int `mapstructure:"max_debug_traces_per_minute"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DebugAttribute: "debug.trace",
		MaxPerMinute:   10,
	}
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.DebugAttribute == "" {
		return errors.New("debug_attribute must not be empty")
	}
	if c.MaxPerMinute <= 0 {
		return fmt.Errorf("max_debug_traces_per_minute must be positive, got %d", c.MaxPerMinute)
	}
	return nil
}
