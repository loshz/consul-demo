package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	consul "github.com/hashicorp/consul/api"
)

// registerConsulService attempts to register a service and health check
// with a local consul agent
func registerConsulService(agent *consul.Agent, svcID, svcAddr string) error {
	// create consul api service
	svc := &consul.AgentServiceRegistration{
		Address: svcAddr,
		ID:      svcID,
		Name:    svcName,
		Tags: []string{
			"demo",
			"api",
		},
		Check: &consul.AgentServiceCheck{
			Interval: "5s",
			HTTP:     fmt.Sprintf("%v/healthz", svcAddr),
			Method:   http.MethodGet,
			Name:     "/healthz",
			Timeout:  "1s",
		},
	}

	// attempt to register new service with local consul agent
	return agent.ServiceRegister(svc)
}

// registerConsulLeader attempts to aquire a lock session with a unique id
func registerConsulLeader(client *consul.Client, stop chan os.Signal, svcID string) error {
	sessionID, _, err := client.Session().Create(&consul.SessionEntry{
		Name:     fmt.Sprintf("service/%s/leader", svcName),
		Behavior: "delete",
		TTL:      "10s",
	}, nil)
	if err != nil {
		return err
	}

	done := make(chan struct{})
	go func() {
		if err := client.Session().RenewPeriodic("10s", sessionID, nil, done); err != nil {
			log.Printf("error renewing lock session: %v", err)
			stop <- os.Kill
			return
		}
	}()

	go func() {
		for {
			leader, _, err := client.KV().Acquire(&consul.KVPair{
				Key:     fmt.Sprintf("service/%s/leader", svcName),
				Value:   []byte(svcID),
				Session: sessionID,
			}, nil)
			if err != nil {
				log.Printf("error acquiring lock: %v", err)
				close(done)
				stop <- os.Kill
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
