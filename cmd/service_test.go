package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	consul "github.com/hashicorp/consul/api"
)

type mockConsulClient struct {
	ServiceRegisterFn func(*consul.AgentServiceRegistration) error
	SessionCreateFn   func(*consul.SessionEntry, *consul.WriteOptions) (string, *consul.WriteMeta, error)
	SessionRenewFn    func(string, *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error)
	KVAcquireFn       func(*consul.KVPair, *consul.WriteOptions) (bool, *consul.WriteMeta, error)
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
	return m.KVAcquireFn(kv, opts)
}

func (m mockConsulClient) SessionCreate(sess *consul.SessionEntry, opts *consul.WriteOptions) (string, *consul.WriteMeta, error) {
	return m.SessionCreateFn(sess, opts)
}

func (m mockConsulClient) SessionRenew(id string, opts *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error) {
	return m.SessionRenewFn(id, opts)
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

func TestRegisterConsulLeader(t *testing.T) {
	// Configure a generic service.
	svc := Service{
		id:    "test",
		ErrCh: make(chan error, 1),
	}

	t.Run("TestSessionCreateError", func(t *testing.T) {
		svc.consul = mockConsulClient{
			SessionCreateFn: func(sess *consul.SessionEntry, opts *consul.WriteOptions) (string, *consul.WriteMeta, error) {
				return "", nil, fmt.Errorf("session create error")
			},
		}

		if err := svc.registerConsulLeader(context.TODO(), 1*time.Millisecond); err == nil {
			t.Error("expected error registering Consul leader, got: nil")
		}
	})

	t.Run("TestSessionRenewError", func(t *testing.T) {
		expected := errors.New("session renew error")

		svc.consul = mockConsulClient{
			SessionCreateFn: func(sess *consul.SessionEntry, opts *consul.WriteOptions) (string, *consul.WriteMeta, error) {
				return "session_id", nil, nil
			},
			SessionRenewFn: func(id string, opts *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error) {
				return nil, nil, expected
			},
		}

		svc.registerConsulLeader(context.TODO(), 1*time.Millisecond)

		if err := <-svc.ErrCh; errors.Unwrap(err) != expected {
			t.Errorf("expected: '%v', got: '%v'", expected, err)
		}
	})

	t.Run("TestKVAcquireError", func(t *testing.T) {
		expected := errors.New("kv acquire error")

		svc.consul = mockConsulClient{
			SessionCreateFn: func(sess *consul.SessionEntry, opts *consul.WriteOptions) (string, *consul.WriteMeta, error) {
				return "session_id", nil, nil
			},
			SessionRenewFn: func(id string, opts *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error) {
				return &consul.SessionEntry{ID: "test"}, nil, nil
			},
			KVAcquireFn: func(*consul.KVPair, *consul.WriteOptions) (bool, *consul.WriteMeta, error) {
				return false, nil, expected
			},
		}

		svc.registerConsulLeader(context.TODO(), 1*time.Millisecond)

		if err := <-svc.ErrCh; errors.Unwrap(err) != expected {
			t.Errorf("expected: '%v', got: '%v'", expected, err)
		}
	})
}

func TestStartSuccess(t *testing.T) {
	// Configure a generic service.
	svc := Service{
		id:    "test",
		addr:  "http://localhost:6000",
		ErrCh: make(chan error, 1),
	}

	svc.consul = mockConsulClient{
		ServiceRegisterFn: func(*consul.AgentServiceRegistration) error {
			return nil
		},
		SessionCreateFn: func(sess *consul.SessionEntry, opts *consul.WriteOptions) (string, *consul.WriteMeta, error) {
			return "session_id", nil, nil
		},
		SessionRenewFn: func(id string, opts *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error) {
			return &consul.SessionEntry{ID: "test"}, nil, nil
		},
		KVAcquireFn: func(*consul.KVPair, *consul.WriteOptions) (bool, *consul.WriteMeta, error) {
			return true, nil, nil
		},
	}

	svc.Start(1 * time.Millisecond)

	// Cancel the context to stop the underlying goroutines.
	svc.cancel()

	// Assert that no errors where received.
	if len(svc.ErrCh) > 0 {
		t.Errorf("expected no errors, got: '%v'", <-svc.ErrCh)
	}
}
