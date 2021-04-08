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
	API         string `toml:"api"`           // api string from lotus, in the form of `token:/ip4/127.0.0.1/tcp/1234/http`
	Wallet      string `toml:"wallet"`        // wallet to be used for making deals
}

type ClientConfig struct {
	Endpoint string `toml:"endpoint"`
}
