package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	EnvDealbotHome = "DEALBOT_HOME"

	// DefaultDaemonListenAddr is a host:port value, where we set up an HTTP endpoint.
	DefaultDaemonListenAddr = "localhost:0"

	DefaultControllerListenAddr = "localhost:8764"
)

func (e *EnvConfig) Load() error {
	// apply fallbacks.
	e.Daemon.Listen = DefaultDaemonListenAddr
	e.Controller.Listen = DefaultControllerListenAddr
	e.Client.Endpoint = "http://" + DefaultControllerListenAddr

	// calculate home directory; use env var, or fall back to $HOME/dealbot
	// otherwise.
	var home string
	if v, ok := os.LookupEnv(EnvDealbotHome); ok {
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

	switch fi, err := os.Stat(home); {
	case os.IsNotExist(err):
		//logging.S().Infof("creating home directory at %s", home)
		if err := os.MkdirAll(home, 0777); err != nil {
			return fmt.Errorf("failed to create home directory at %s: %w", home, err)
		}
	case err == nil:
		//logging.S().Infof("using home directory: %s", home)
	case !fi.IsDir():
		return fmt.Errorf("home path is not a directory %s", home)
	}

	return nil
}
