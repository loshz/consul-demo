package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	consul "github.com/hashicorp/consul/api"
)

const (
	svcName = "consul-demo"
)

func main() {
	// configure command line flags
	id := flag.String("id", "", "Unique service ID")
	fail := flag.Bool("fail", false, "Force /healthz failure")
	port := flag.Int("port", 6000, "TCP/IP port number the server listens on")
	flag.Parse()

	if id == nil {
		log.Fatal("unique service id must be set via -id=XXX")
	}

	// configure service specific vars
	svcID := fmt.Sprintf("%s-%s", svcName, *id)
	svcAddr := fmt.Sprintf("http://%v:%v", svcID, *port)

	// configure channel for receiving stop signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// configure and start HTTP server with given port
	server := newHTTPServer(*port, *fail, stop)

	// configure consul client
	config := &consul.Config{
		Address: "consul-agent:8500",
	}
	client, err := consul.NewClient(config)
	if err != nil {
		log.Fatalf("error creating consul client: %v", err)
	}

	// attempt to register new service with local consul agent
	if err := registerConsulService(client.Agent(), svcID, svcAddr); err != nil {
		log.Fatalf("error registering service with consul: %v", err)
	}

	// attempt to register as leader and start background tasks
	if err := registerConsulLeader(client, stop, svcID); err != nil {
		log.Fatalf("error registering service as leader: %v", err)
	}

	// wait for a stop signal
	<-stop
	log.Println("received stop signal")

	// attempt to gracefully shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("error shutting down http server: %v", err)
	}

	// attempt to deregister service on shutdown
	if err := client.Agent().ServiceDeregister(svcID); err != nil {
		log.Fatalf("error deregistering service from consul: %v", err)
	}

	log.Println("successfully stopped http server and other background tasks")
}
