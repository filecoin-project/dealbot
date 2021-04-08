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
	Listen      string `toml:"listen"`
	DataDir     string `toml:"data_dir"`      // writable directory used to transfer data to node
	NodeDataDir string `toml:"node_data_dir"` // data-dir from relative to node's location
}

type ClientConfig struct {
	Endpoint string `toml:"endpoint"`
}
