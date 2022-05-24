package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const svcName = "consul-demo"

func main() {
	id := flag.String("id", "", "Unique service ID")
	port := flag.Int("port", 6000, "TCP/IP port number the server listens on")
	flag.Parse()

	if id == nil {
		log.Fatal("unique service id must be set via -id=XXX")
	}

	svcID := fmt.Sprintf("%s-%s", svcName, *id)

	// Configure channel for receiving stop signals.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Configure a new service and start background tasks.
	svc, err := NewService(svcID, *port, stop)
	if err != nil {
		log.Fatalf("critical service error: %v", err)
	}
	svc.Start()

	// wait for a stop signal
	<-stop
	log.Println("received stop signal")

	// Attempt graceful shutdown.
	if err := svc.Shutdown(); err != nil {
		log.Fatalf("critical shutdown error: %v", err)
	}

	log.Println("successfully stopped http server and other background tasks")
}
