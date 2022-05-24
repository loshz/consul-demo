package main

import consul "github.com/hashicorp/consul/api"

const ConsulAgentAddr = "consul-agent:8500"

type ConsulClient interface {
	Agent() *consul.Agent
	Catalog() *consul.Catalog
	KV() *consul.KV
	Session() *consul.Session
}

type ConsulAgent interface {
	ServiceDeregister(id string) error
	ServiceRegister(*consul.AgentServiceRegistration) error
}

type ConsulCatalog interface {
	Service(service, tag string, opts *consul.QueryOptions) ([]*consul.CatalogService, *consul.QueryMeta, error)
	Services(*consul.QueryOptions) (map[string][]string, *consul.QueryMeta, error)
}

type ConsulKV interface {
	Acquire(*consul.KVPair, *consul.WriteOptions) (bool, *consul.WriteMeta, error)
}

type ConsulSession interface {
	Create(*consul.SessionEntry, *consul.WriteOptions) (string, *consul.WriteMeta, error)
	RenewPeriodic(initialTTL, id string, opts *consul.WriteOptions, doneCh <-chan struct{}) error
}
