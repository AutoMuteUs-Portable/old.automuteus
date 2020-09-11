package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func load() error {
	// Load .env file
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal(err)
		return err
	}

	// Read bytes
	bytes, err := ioutil.ReadFile("diffenvs.json")
	if err != nil {
		log.Fatal(err)
		return err
	}

	var DifferentEnvs map[string]([]map[string]string)

	json.Unmarshal(bytes, &DifferentEnvs)

	if DifferentEnvs == nil {
		return nil
	}

	for _, i := range DifferentEnvs["automuteus"] {
		for key, value := range i {
			os.Setenv(key, os.Getenv(value))
		}
	}

	return nil
}
