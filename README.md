# Consul Demo
[![Build Status](https://github.com/loshz/consul-demo/workflows/ci/badge.svg)](https://github.com/loshz/consul-demo/actions)

This repo serves as a basis for writing Go services that use [HashiCorp Consul](https://www.consul.io/) for service discovery, amongst other things.

## Usage
The included `docker-compose.yml` creates a Consul cluster with a single server and agent. It also starts 3 replicas of the included Go service:
```bash
$ docker compose up
```

## Go Service
The included Go service found in the `./cmd` directory performs the following tasks:
- Auto registers with local Consul Agent including a HTTP health check.
- Automatic leader election using Consul's session locking functionality.
- Automatic service discovery using Consul's Catalog functionality.
