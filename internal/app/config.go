package app

import (
	"mistletoe/internal/config"
)

// Config is a type alias for config.Config.
type Config = config.Config

// Repository is a type alias for config.Repository.
type Repository = config.Repository

var (
	// ParseConfig is a wrapper for config.ParseConfig.
	ParseConfig = config.ParseConfig
	// GetRepoDirName is a wrapper for config.GetRepoDirName.
	GetRepoDirName = config.GetRepoDirName
)

// Wrapper functions to use the new package
func loadConfigData(data []byte) (*config.Config, error) {
	return config.LoadConfigData(data)
}

func loadConfigFile(configFile string) (*config.Config, error) {
	return config.LoadConfigFile(configFile)
}
