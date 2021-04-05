package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	EnvDealbotHome = "DEALBOT_HOME"

	// DefaultListenAddr is a host:port value, where we set up an HTTP endpoint.
	DefaultListenAddr = "localhost:8765"

	// DefaultClientURL is the HTTP(S) endpoint of the server.
	DefaultClientURL = "http://" + DefaultListenAddr
)

func (e *EnvConfig) Load() error {
	// apply fallbacks.
	e.Daemon.Listen = DefaultListenAddr
	e.Client.Endpoint = DefaultClientURL

	// calculate home directory; use env var, or fall back to $HOME/testground
	// otherwise.
	var home string
	if v, ok := os.LookupEnv(EnvDealbotHome); ok {
		// we have an env var.
		home = v
	} else {
		// fallback to $HOME/testground.
		v, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to obtain user home dir: %w", err)
		}
		home = filepath.Join(v, "testground")
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

	// parse the .env.toml file, if it exists.
	//f := filepath.Join(e.dirs.Home(), ".env.toml")
	//if _, err := os.Stat(f); err == nil {
	//// try to load the optional .env.toml file
	//_, err = toml.DecodeFile(f, e)
	//if err != nil {
	//return fmt.Errorf("found .env.toml at %s, but failed to parse: %w", f, err)
	//}
	////logging.S().Infof(".env.toml loaded from: %s", f)
	//} else {
	////logging.S().Infof("no .env.toml found at %s; running with defaults", f)
	//}
	return nil
}
