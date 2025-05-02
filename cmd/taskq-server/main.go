package main

import (
	"fmt"
	"log"

	"github.com/Devashish08/taskq/internal/api"
)

func main() {
	fmt.Println("Starting TaskQ server...")

	router := api.SetupRouter()
	// TODO: Initialize config loading
	// TODO: initialize workers

	// start the server

	// TODO: mark the port configurable later

	part := "8080"
	fmt.Printf("Server listening on port %s\n", part)
	if err := router.Run(":" + part); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
