package main

import (
	"RCGOTCP/internal/apiserver"
	"log"
)

func main() {
	if err := apiserver.Start(); err != nil {
		log.Fatal(err)
	}
}
