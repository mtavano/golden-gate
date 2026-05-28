package config

type Config struct {
	// Global
	Environment  string `envconfig:"environment" default:"development"`
	DBDriver     string `envconfig:"db_driver" default:"sqlite"`
	DBPath       string `envconfig:"db_path" default:"./data/golden_gate.db"`
	ConfigPath   string `envconfig:"config_path" default:"./configs/service.json"`
	TimeZone     string `envconfig:"time_zone" default:"America/Santiago"`
	Port         int    `envconfig:"port" default:"8080"`
	MaxBodyBytes int    `envconfig:"max_body_bytes" default:"1048576"`

	// Editor (dashboard /config) basic-auth credentials
	EditorUser string `envconfig:"editor_user" default:"admin"`
	EditorPass string `envconfig:"editor_pass" default:"supersecurepassword"`
}
