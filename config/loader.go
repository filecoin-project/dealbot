package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/urfave/cli/v2"
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

// OverrideFromEnv sets config fields based on environmental variables
// of the format DEALBOT_DAEMON_LISTEN
func (e *EnvConfig) OverrideFromEnv(c *cli.Context) error {
	v := reflect.ValueOf(e)
	envConfType := v.Type()

	// for daemon/controller/client...
	for i := 0; i < v.NumField(); i++ {
		subConfType := v.Field(i).Type()
		// for fields within each sub-config
		for j := 0; j < v.Field(i).NumField(); j++ {
			if val, ok := os.LookupEnv(fmt.Sprintf("DEALBOT_%s_%s", envConfType.Field(i).Name, subConfType.Field(j).Name)); ok {
				switch v.Field(i).Field(j).Type().Kind() {
				case reflect.String:
					v.Field(i).Field(j).SetString(val)
				default:
					return fmt.Errorf("Unexpected type %s", v.Field(i).Field(j).Type().Kind())
				}
			}
		}
	}
	return nil
}
