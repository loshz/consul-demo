package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	id := flag.String("id", "", "Unique service ID")
	port := flag.Int("port", 6000, "TCP/IP port number the server listens on")
	flag.Parse()

	if id == nil {
		log.Fatal("unique service id must be set via -id=XXX")
	}

	// Configure channel for receiving stop signals.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Configure a new service and start background tasks.
	svc, err := NewService(*id, *port)
	if err != nil {
		log.Fatalf("critical service error: %v", err)
	}
	svc.Start(defaultTickDuration)

	// Wait for a stop signal or service error.
	select {
	case err := <-svc.ErrCh:
		log.Fatalf("critical service error: %v", err)
	case <-stop:
		log.Println("received stop signal")

		// Attempt graceful shutdown.
		if err := svc.Shutdown(); err != nil {
			log.Fatalf("critical shutdown error: %v", err)
		}

		log.Println("successfully stopped http server and other background tasks")
	}
}
