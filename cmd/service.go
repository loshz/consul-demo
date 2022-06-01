package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

	// Context used to control active goroutines.
	cancel context.CancelFunc

	// Channel for sending runtime errors.
	ErrCh chan error
}

// NewService configures a Service Consul client and local HTTP server for health checks.
func NewService(id string, port int) (*Service, error) {
	// Configure consul client.
	client, err := NewConsul()
	if err != nil {
		return nil, fmt.Errorf("error creating consul client: %v", err)
	}

	id = fmt.Sprintf("%s-%s", svcName, id)

	s := &Service{
		id:     id,
		addr:   fmt.Sprintf("http://%v:%v", id, port),
		consul: client,
		ErrCh:  make(chan error, 1),
	}

	// Start local HTTP server.
	s.server = NewHTTPServer(fmt.Sprintf("%s:%d", id, port), s.ErrCh)

	return s, nil
}

// Start service registration, leader election and service discovery in the background.
func (s *Service) Start(dur time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Start local HTTP server.
	s.startHTTP()

	// attempt to register new service with local consul agent
	if err := s.registerConsulService(); err != nil {
		log.Fatalf("error registering service with consul: %v", err)
	}

	// attempt to register as leader and start background tasks
	if err := s.registerConsulLeader(ctx, dur); err != nil {
		log.Fatalf("error registering service as leader: %v", err)
	}

	s.getRegisteredConsulServices(ctx, dur)
}

// Shutdown the background tasks gracefully.
func (s *Service) Shutdown() error {
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

func (s *Service) startHTTP() {
	log.Printf("starting http server, addr: %s", s.addr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			s.ErrCh <- fmt.Errorf("encountered critical error from HTTP server: %v", err)
			return
		}
	}()
}

// registerConsulService attempts to register a service and health check
// with a local consul agent.
func (s *Service) registerConsulService() error {
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
			HTTP:     fmt.Sprintf("%s%s", s.addr, healthzURI),
			Method:   http.MethodGet,
			Name:     healthzURI,
			Timeout:  "1s",
		},
	}

	// attempt to register new service with local consul agent
	return s.consul.AgentServiceRegister(svc)
}

// registerConsulLeader attempts to aquire a lock session with a unique id.
func (s *Service) registerConsulLeader(ctx context.Context, dur time.Duration) error {
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
				s.ErrCh <- fmt.Errorf("error renewing leader session: %w", err)
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
				s.ErrCh <- fmt.Errorf("error acquiring lock: %w", err)
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

// getRegisteredConsulServices periodically polls the Consul catalog for registered services.
// It will only return services with passing health checks.
func (s *Service) getRegisteredConsulServices(ctx context.Context, dur time.Duration) {
	t := time.NewTicker(dur)

	go func() {
		for {
			select {
			case <-t.C:
				// Get a list of all registered services.
				svcs, _, err := s.consul.CatalogServices(nil)
				if err != nil {
					s.ErrCh <- fmt.Errorf("failed to get registered consul services: %w", err)
					return
				}

				for svc := range svcs {
					// Only query services of the same type.
					if svc == svcName {
						// Get individual service info.
						catSvcs, _, err := s.consul.CatalogService(svc, "", nil)
						if err != nil {
							s.ErrCh <- fmt.Errorf("failed to get registered consul service details: %w", err)
							return
						}

						for _, catSvc := range catSvcs {
							if catSvc.ServiceID != s.id && catSvc.Checks.AggregatedStatus() == consul.HealthPassing {
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
