package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	consul "github.com/hashicorp/consul/api"
)

const svcName = "consul-demo"

type Service struct {
	// Unique Service ID.
	id string

	// Unique Servie address.
	addr string

	// Consul client.
	consul ConsulClient

	// Local HTTP server.
	server *http.Server

	// Channel for sending stop commands.
	stop chan os.Signal
}

func NewService(id string, port int, stop chan os.Signal) (Service, error) {
	s := Service{
		id:   fmt.Sprintf("%s-%s", svcName, id),
		addr: fmt.Sprintf("http://%v:%v", id, port),
		stop: stop,
	}

	// configure consul client
	config := &consul.Config{
		Address: ConsulAgentAddr,
	}
	var err error
	s.consul, err = consul.NewClient(config)
	if err != nil {
		return s, fmt.Errorf("error creating consul client: %v", err)
	}

	// Start local HTTP server.
	s.server = NewHTTPServer(port, stop)

	return s, nil
}

func (s Service) Start() {
	// attempt to register new service with local consul agent
	if err := s.registerConsulService(); err != nil {
		log.Fatalf("error registering service with consul: %v", err)
	}

	// attempt to register as leader and start background tasks
	if err := s.registerConsulLeader(); err != nil {
		log.Fatalf("error registering service as leader: %v", err)
	}

	s.getRegisteredConsulServices()
}

func (s Service) Shutdown() error {
	// attempt to deregister service on shutdown
	if err := s.consul.Agent().ServiceDeregister(s.id); err != nil {
		return fmt.Errorf("error deregistering service from consul: %v", err)
	}

	// attempt to gracefully shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("error shutting down http server: %v", err)
	}

	return nil
}

// registerConsulService attempts to register a service and health check
// with a local consul agent
func (s Service) registerConsulService() error {
	// create consul api service
	svc := &consul.AgentServiceRegistration{
		Address: s.addr,
		ID:      s.id,
		Name:    svcName,
		Tags: []string{
			"demo",
			"api",
		},
		Check: &consul.AgentServiceCheck{
			Interval: "5s",
			HTTP:     fmt.Sprintf("%v/healthz", s.addr),
			Method:   http.MethodGet,
			Name:     "/healthz",
			Timeout:  "1s",
		},
	}

	// attempt to register new service with local consul agent
	return s.consul.Agent().ServiceRegister(svc)
}

// registerConsulLeader attempts to aquire a lock session with a unique id
func (s Service) registerConsulLeader() error {
	sessionID, _, err := s.consul.Session().Create(&consul.SessionEntry{
		Name:     fmt.Sprintf("service/%s/leader", svcName),
		Behavior: "delete",
		TTL:      "10s",
	}, nil)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		if err := s.consul.Session().RenewPeriodic("10s", sessionID, nil, done); err != nil {
			log.Printf("error renewing lock session: %v", err)
			s.stop <- os.Kill
			return
		}
	}()

	go func() {
		for {
			leader, _, err := s.consul.KV().Acquire(&consul.KVPair{
				Key:     fmt.Sprintf("service/%s/leader", svcName),
				Value:   []byte(s.id),
				Session: sessionID,
			}, nil)
			if err != nil {
				log.Printf("error acquiring lock: %v", err)
				close(done)
				s.stop <- os.Kill
				return
			}

			if leader {
				log.Println("lock acquired, registered as leader")
			}

			time.Sleep(10 * time.Second)
		}
	}()

	return nil
}

// getRegisteredConsulServices periodically polls the Consul catalog for new services.
func (s Service) getRegisteredConsulServices() {
	go func() {
		catalog := s.consul.Catalog()

		for {
			opts := &consul.QueryOptions{}
			svcs, _, err := catalog.Services(opts)
			if err != nil {
				log.Println("failed to get service catalog, will retry")
			}

			for svc := range svcs {
				if svc == svcName {
					catSvcs, _, err := catalog.Service(svc, "", opts)
					if err != nil {
						log.Printf("failed to get service details: %s", err)
						continue
					}

					for _, catSvc := range catSvcs {
						if catSvc.ServiceID != s.id {
							log.Printf("discovered new service: %s", catSvc.ServiceID)
						}
					}
				}
			}

			time.Sleep(30 * time.Second)
		}
	}()
}
