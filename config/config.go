package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

var config map[string]string

func Load() {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Open(dir + string(filepath.Separator) + "config.json")
	if err != nil {
		panic(err.Error())
	}

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		panic(err.Error())
	}
}

func Get(name string) string {
	return config[name]
}
