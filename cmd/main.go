package main

import (
	"RCGOTCP/internal/apiserver"
	"log"
)

func main() { //todo: add configuration
	if err := apiserver.Start(); err != nil {
		log.Fatal(err)
	}
}
