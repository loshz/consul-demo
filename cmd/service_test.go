package main

import (
	"fmt"
	"testing"

	consul "github.com/hashicorp/consul/api"
)

type mockConsulClient struct {
	ServiceRegisterFn func(*consul.AgentServiceRegistration) error
}

func (m mockConsulClient) AgentServiceDeregister(id string) error {
	return nil
}

func (m mockConsulClient) AgentServiceRegister(svc *consul.AgentServiceRegistration) error {
	return m.ServiceRegisterFn(svc)
}

func (m mockConsulClient) CatalogService(service, tag string, opts *consul.QueryOptions) ([]*consul.CatalogService, *consul.QueryMeta, error) {
	return nil, nil, nil
}

func (m mockConsulClient) CatalogServices(opts *consul.QueryOptions) (map[string][]string, *consul.QueryMeta, error) {
	return nil, nil, nil
}

func (m mockConsulClient) KVAcquire(kv *consul.KVPair, opts *consul.WriteOptions) (bool, *consul.WriteMeta, error) {
	return false, nil, nil
}

func (m mockConsulClient) SessionCreate(sess *consul.SessionEntry, opts *consul.WriteOptions) (string, *consul.WriteMeta, error) {
	return "", nil, nil
}

func (m mockConsulClient) SessionRenew(id string, opts *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error) {
	return nil, nil, nil
}

func TestRegisterConsulService(t *testing.T) {
	// Configure a generic service.
	svc := Service{
		id:   "test",
		addr: "http://localhost:6000",
	}

	// Assert that an error during service registration is handled.
	t.Run("TestRegisterError", func(t *testing.T) {
		svc.consul = mockConsulClient{
			ServiceRegisterFn: func(*consul.AgentServiceRegistration) error {
				return fmt.Errorf("register error")
			},
		}

		if err := svc.registerConsulService(); err == nil {
			t.Error("expected error registering service with Consul, got: nil")
		}
	})

	// Assert that a successful service registration results in no errors.
	t.Run("TestRegisterSuccess", func(t *testing.T) {
		svc.consul = mockConsulClient{
			ServiceRegisterFn: func(*consul.AgentServiceRegistration) error {
				return nil
			},
		}

		if err := svc.registerConsulService(); err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})
}
