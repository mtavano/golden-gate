package config

type Config struct {
	// Global
	Environment string `envconfig:"environment" default:"development"`
	DBDriver    string `envconfig:"db_driver" default:"sqlite"`
	DBPath      string `envconfig:"db_path" default:"./data/golden_gate.db"`
	TimeZone    string `envconfig:"time_zone" default:"America/Santiago"`
	Port        int    `envconfig:"port" default:"8080"`
}
