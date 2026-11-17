package config

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// KeyDefinedIn reports whether the TOML file at path sets key, in the dotted
// form SetProjectConfigValues takes ("session.open_in"). A missing file sets
// nothing.
func KeyDefinedIn(path, key string) (bool, error) {
	var v map[string]any
	md, err := toml.DecodeFile(path, &v)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return md.IsDefined(strings.Split(key, ".")...), nil
}

// KeyDefinedInFiles reports whether the global or the project config file
// sets key: when neither does, grove uses the built-in default for it.
func KeyDefinedInFiles(key string) (bool, error) {
	globalPath, projectPath, err := GetConfigPaths()
	if err != nil {
		return false, err
	}
	if envConfig := os.Getenv("GROVE_CONFIG"); envConfig != "" {
		globalPath = envConfig
	}
	for _, p := range []string{globalPath, projectPath} {
		if p == "" {
			continue
		}
		if ok, err := KeyDefinedIn(p, key); ok || err != nil {
			return ok, err
		}
	}
	return false, nil
}
