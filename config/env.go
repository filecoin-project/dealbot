package config

type EnvConfig struct {
	Daemon DaemonConfig `toml:"daemon"`
	Client ClientConfig `toml:"client"`
}

type DaemonConfig struct {
	Listen string `toml:"listen"`
}

type ClientConfig struct {
	Endpoint string `toml:"endpoint"`
}
