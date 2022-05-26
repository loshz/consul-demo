package main

import (
	"fmt"

	consul "github.com/hashicorp/consul/api"
)

const ConsulAgentAddr = "consul-agent:8500"

type ConsulClient interface {
	// Wrapper around client.Agent()
	AgentServiceDeregister(id string) error
	AgentServiceRegister(*consul.AgentServiceRegistration) error

	// Wrapper around client.Catalog()
	CatalogService(service, tag string, opts *consul.QueryOptions) ([]*consul.CatalogService, *consul.QueryMeta, error)
	CatalogServices(*consul.QueryOptions) (map[string][]string, *consul.QueryMeta, error)

	// Wrapper around client.KV()
	KVAcquire(*consul.KVPair, *consul.WriteOptions) (bool, *consul.WriteMeta, error)

	// Wrapper around client.Session()
	SessionCreate(*consul.SessionEntry, *consul.WriteOptions) (string, *consul.WriteMeta, error)
	SessionRenew(id string, opts *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error)
}

type Consul struct {
	client *consul.Client
}

func NewConsul() (Consul, error) {
	config := &consul.Config{
		Address: ConsulAgentAddr,
	}
	client, err := consul.NewClient(config)
	if err != nil {
		return Consul{}, fmt.Errorf("error creating consul client: %v", err)
	}

	return Consul{client}, nil
}

func (c Consul) AgentServiceDeregister(id string) error {
	return c.client.Agent().ServiceDeregister(id)
}

func (c Consul) AgentServiceRegister(svc *consul.AgentServiceRegistration) error {
	return c.client.Agent().ServiceRegister(svc)
}

func (c Consul) CatalogService(service, tag string, opts *consul.QueryOptions) ([]*consul.CatalogService, *consul.QueryMeta, error) {
	return c.client.Catalog().Service(service, tag, opts)
}

func (c Consul) CatalogServices(opts *consul.QueryOptions) (map[string][]string, *consul.QueryMeta, error) {
	return c.client.Catalog().Services(opts)
}

func (c Consul) KVAcquire(kv *consul.KVPair, opts *consul.WriteOptions) (bool, *consul.WriteMeta, error) {
	return c.client.KV().Acquire(kv, opts)
}

func (c Consul) SessionCreate(sess *consul.SessionEntry, opts *consul.WriteOptions) (string, *consul.WriteMeta, error) {
	return c.client.Session().Create(sess, opts)
}

func (c Consul) SessionRenew(id string, opts *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error) {
	return c.client.Session().Renew(id, opts)
}
