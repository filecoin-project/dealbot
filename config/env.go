package config

type EnvConfig struct {
	Daemon     DaemonConfig     `toml:"daemon"`
	Controller ControllerConfig `toml:"controller"`
	Client     ClientConfig     `toml:"client"`
}

type ControllerConfig struct {
	Listen string `toml:"listen"`
}

type DaemonConfig struct {
	Listen string `toml:"listen"`
}

type ClientConfig struct {
	Endpoint string `toml:"endpoint"`
}
