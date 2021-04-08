package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	EnvDealbotHome = "DEALBOT_HOME"

	// DefaultDaemonListenAddr is a host:port value, where we set up an HTTP endpoint.
	DefaultDaemonListenAddr = "localhost:0"

	DefaultControllerListenAddr = "localhost:8764"
)

func (e *EnvConfig) Load(configpath string) error {
	// apply fallbacks.
	e.Daemon.Listen = DefaultDaemonListenAddr
	e.Controller.Listen = DefaultControllerListenAddr
	e.Client.Endpoint = "http://" + DefaultControllerListenAddr

	// calculate home directory; use env var, or fall back to $HOME/dealbot
	// otherwise.
	var home string
	var f string
	if configpath != "" {
		f = configpath
	} else if v, ok := os.LookupEnv(EnvDealbotHome); ok {
		// we have an env var.
		home = v
	} else {
		// fallback to $HOME/dealbot.
		v, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to obtain user home dir: %w", err)
		}
		home = filepath.Join(v, "dealbot")
	}

	if f == "" {
		switch fi, err := os.Stat(home); {
		case os.IsNotExist(err):
			if err := os.MkdirAll(home, 0777); err != nil {
				return fmt.Errorf("failed to create home directory at %s: %w", home, err)
			}
		case err == nil:
		case !fi.IsDir():
			return fmt.Errorf("home path is not a directory %s", home)
		}

		// parse the .env.toml file, if it exists.
		f = filepath.Join(home, ".env.toml")
	}

	if _, err := os.Stat(f); err == nil {
		// try to load the optional .env.toml file
		_, err = toml.DecodeFile(f, e)
		if err != nil {
			return fmt.Errorf("found .env.toml at %s, but failed to parse: %w", f, err)
		}
	}

	return nil
}
