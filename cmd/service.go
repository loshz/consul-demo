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

const (
	svcName = "consul-demo"

	defaultTickDuration = 30 * time.Second
)

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

	// Context used to control active goroutines.
	cancel context.CancelFunc
}

// NewService configures a Service Consul client and local HTTP server for health checks.
func NewService(id string, port int, stop chan os.Signal) (Service, error) {
	id = fmt.Sprintf("%s-%s", svcName, id)

	s := Service{
		id:   id,
		addr: fmt.Sprintf("http://%v:%v", id, port),
		stop: stop,
	}

	// configure consul client
	var err error
	s.consul, err = NewConsul()
	if err != nil {
		return s, fmt.Errorf("error creating consul client: %v", err)
	}

	// Start local HTTP server.
	s.server = NewHTTPServer(port, stop)

	return s, nil
}

// Start service registration, leader election and service discovery in the background.
func (s Service) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// attempt to register new service with local consul agent
	if err := s.registerConsulService(); err != nil {
		log.Fatalf("error registering service with consul: %v", err)
	}

	// attempt to register as leader and start background tasks
	if err := s.registerConsulLeader(ctx, defaultTickDuration); err != nil {
		log.Fatalf("error registering service as leader: %v", err)
	}

	s.getRegisteredConsulServices(ctx, defaultTickDuration)
}

// Shutdown the background tasks gracefully.
func (s Service) Shutdown() error {
	// Signal that goroutines started by the service should be cancelled.
	s.cancel()

	// attempt to deregister service on shutdown
	if err := s.consul.AgentServiceDeregister(s.id); err != nil {
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
// with a local consul agent.
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
	return s.consul.AgentServiceRegister(svc)
}

// registerConsulLeader attempts to aquire a lock session with a unique id.
func (s Service) registerConsulLeader(ctx context.Context, dur time.Duration) error {
	sessionID, _, err := s.consul.SessionCreate(&consul.SessionEntry{
		Name:     fmt.Sprintf("service/%s/leader", svcName),
		Behavior: consul.SessionBehaviorDelete,
		TTL:      "60s",
	}, nil)
	if err != nil {
		return err
	}

	t := time.NewTicker(dur)
	go func() {
		// Keep track of the current Session ID.
		sID := sessionID

		for {
			// Attempt to renew the session before acquiring the lock.
			session, _, err := s.consul.SessionRenew(sID, nil)
			if err != nil {
				log.Printf("error renewing leader session: %v", err)
				s.stop <- os.Kill
				return
			}

			// Check if the session exists.
			if session == nil {
				log.Printf("error: session does not exist")
				continue
			}
			sID = session.ID

			leader, _, err := s.consul.KVAcquire(&consul.KVPair{
				Key:     fmt.Sprintf("service/%s/leader", svcName),
				Value:   []byte(s.id),
				Session: sID,
			}, nil)
			if err != nil {
				log.Printf("error acquiring lock: %v", err)
				s.stop <- os.Kill
				return
			}

			if leader {
				log.Println("lock acquired, registered as leader")
			}

			select {
			case <-t.C:
				continue
			case <-ctx.Done():
				// Exit this goroutine during service shutdown in order to
				// free resources.
				return
			}
		}
	}()

	return nil
}

// getRegisteredConsulServices periodically polls the Consul catalog for new services.
func (s Service) getRegisteredConsulServices(ctx context.Context, dur time.Duration) {
	t := time.NewTicker(dur)
	go func() {
		for {
			select {
			case <-t.C:
				opts := &consul.QueryOptions{}
				svcs, _, err := s.consul.CatalogServices(opts)
				if err != nil {
					log.Println("failed to get service catalog, will retry")
				}

				for svc := range svcs {
					if svc == svcName {
						catSvcs, _, err := s.consul.CatalogService(svc, "", opts)
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
			case <-ctx.Done():
				// Exit this goroutine during service shutdown in order to
				// free resources.
				return
			}
		}
	}()
}
