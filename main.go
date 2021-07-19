package main

import (
	"github.com/BurntSushi/toml"
	"github.com/bulbetski/kvstorage-srv/api"
	"log"
)

func main() {
	config := api.NewConfig()
	_, err := toml.DecodeFile("configs/db_conf.toml", config)
	if err != nil {
		log.Fatal(err)
	}

	if err := api.Start(config); err != nil {
		log.Fatal(err)
	}
}
