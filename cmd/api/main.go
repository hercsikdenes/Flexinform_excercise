package main

import (
	"log"
	"os"

	"carservice/internal/application"
)

func main() {
	logger := log.New(os.Stdout, "carservice ", log.LstdFlags|log.LUTC)
	if err := application.Run(logger); err != nil {
		logger.Printf("application stopped with an error: %v", err)
		os.Exit(1)
	}
}
