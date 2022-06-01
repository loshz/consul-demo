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
	ServiceRegisterFn func() error
	CatalogServiceFn  func() ([]*consul.CatalogService, *consul.QueryMeta, error)
	CatalogServicesFn func() (map[string][]string, *consul.QueryMeta, error)
	KVAcquireFn       func() (bool, *consul.WriteMeta, error)
	SessionCreateFn   func() (string, *consul.WriteMeta, error)
	SessionRenewFn    func() (*consul.SessionEntry, *consul.WriteMeta, error)
}

func (m mockConsulClient) AgentServiceDeregister(id string) error {
	return nil
}

func (m mockConsulClient) AgentServiceRegister(*consul.AgentServiceRegistration) error {
	return m.ServiceRegisterFn()
}

func (m mockConsulClient) CatalogService(service, tag string, opts *consul.QueryOptions) ([]*consul.CatalogService, *consul.QueryMeta, error) {
	return m.CatalogServiceFn()
}

func (m mockConsulClient) CatalogServices(*consul.QueryOptions) (map[string][]string, *consul.QueryMeta, error) {
	return m.CatalogServicesFn()
}

func (m mockConsulClient) KVAcquire(*consul.KVPair, *consul.WriteOptions) (bool, *consul.WriteMeta, error) {
	return m.KVAcquireFn()
}

func (m mockConsulClient) SessionCreate(*consul.SessionEntry, *consul.WriteOptions) (string, *consul.WriteMeta, error) {
	return m.SessionCreateFn()
}

func (m mockConsulClient) SessionRenew(string, *consul.WriteOptions) (*consul.SessionEntry, *consul.WriteMeta, error) {
	return m.SessionRenewFn()
}

func TestRegisterConsulService(t *testing.T) {
	// Configure a generic service.
	svc, _ := NewService("test", 8001)

	// Assert that an error during service registration is handled.
	t.Run("TestRegisterError", func(t *testing.T) {
		svc.consul = mockConsulClient{
			ServiceRegisterFn: func() error {
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
			ServiceRegisterFn: func() error {
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
	svc, _ := NewService("test", 8001)

	t.Run("TestSessionCreateError", func(t *testing.T) {
		svc.consul = mockConsulClient{
			SessionCreateFn: func() (string, *consul.WriteMeta, error) {
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
			SessionCreateFn: func() (string, *consul.WriteMeta, error) {
				return "session_id", nil, nil
			},
			SessionRenewFn: func() (*consul.SessionEntry, *consul.WriteMeta, error) {
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
			SessionCreateFn: func() (string, *consul.WriteMeta, error) {
				return "session_id", nil, nil
			},
			SessionRenewFn: func() (*consul.SessionEntry, *consul.WriteMeta, error) {
				return &consul.SessionEntry{ID: "test"}, nil, nil
			},
			KVAcquireFn: func() (bool, *consul.WriteMeta, error) {
				return false, nil, expected
			},
		}

		svc.registerConsulLeader(context.TODO(), 1*time.Millisecond)

		if err := <-svc.ErrCh; errors.Unwrap(err) != expected {
			t.Errorf("expected: '%v', got: '%v'", expected, err)
		}
	})
}

func TestGetRegisteredConsulServices(t *testing.T) {
	// Configure a generic service.
	svc, _ := NewService("test", 8001)

	// Mocked Consul services.
	svcs := map[string][]string{
		svcName: nil,
	}

	t.Run("TestCatalogServicesError", func(t *testing.T) {
		svc.consul = mockConsulClient{
			CatalogServicesFn: func() (map[string][]string, *consul.QueryMeta, error) {
				return nil, nil, errors.New("catalog services error")
			},
		}

		svc.getRegisteredConsulServices(context.TODO(), 1*time.Millisecond)

		if err := <-svc.ErrCh; err == nil {
			t.Error("expected error getting registered consul services")
		}
	})

	t.Run("TestCatalogServiceError", func(t *testing.T) {
		svc.consul = mockConsulClient{
			CatalogServicesFn: func() (map[string][]string, *consul.QueryMeta, error) {
				return svcs, nil, nil
			},
			CatalogServiceFn: func() ([]*consul.CatalogService, *consul.QueryMeta, error) {
				return nil, nil, errors.New("catalog service error")
			},
		}

		svc.getRegisteredConsulServices(context.TODO(), 1*time.Millisecond)

		if err := <-svc.ErrCh; err == nil {
			t.Error("expected error getting registered consul services")
		}
	})

	t.Run("TestSuccess", func(t *testing.T) {
		svc.consul = mockConsulClient{
			CatalogServicesFn: func() (map[string][]string, *consul.QueryMeta, error) {
				return svcs, nil, nil
			},
			CatalogServiceFn: func() ([]*consul.CatalogService, *consul.QueryMeta, error) {
				return nil, nil, nil
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// Let the service run a couple of times.
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		svc.getRegisteredConsulServices(ctx, 1*time.Millisecond)

		// Assert that no errors where received.
		if len(svc.ErrCh) > 0 {
			t.Errorf("expected no errors, got: '%v'", <-svc.ErrCh)
		}
	})
}

func TestStartSuccess(t *testing.T) {
	// Configure a generic service.
	svc, _ := NewService("test", 8001)

	svc.consul = mockConsulClient{
		ServiceRegisterFn: func() error {
			return nil
		},
		SessionCreateFn: func() (string, *consul.WriteMeta, error) {
			return "session_id", nil, nil
		},
		SessionRenewFn: func() (*consul.SessionEntry, *consul.WriteMeta, error) {
			return &consul.SessionEntry{ID: "test"}, nil, nil
		},
		KVAcquireFn: func() (bool, *consul.WriteMeta, error) {
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
