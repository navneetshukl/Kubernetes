# Kubernetes — Task Management System

> 🚧 **Work in progress.** This repo is being built as a hands-on playground for learning Kubernetes concepts (multi-container pods, sidecars, init containers, ConfigMaps/Secrets, Services) using a small Go microservices system. More manifests, docs, and features will be added in the coming days.

## Overview

A minimal task-management system split into independent Go services, containerized and deployed on Kubernetes. It's designed to demonstrate real Kubernetes patterns rather than just "hello world" pods:

- **Init containers** for running DB migrations before the API starts
- **The sidecar pattern** for log streaming via a shared volume
- **ConfigMaps & Secrets** for app/database configuration
- **ClusterIP Services** for internal service-to-service communication

## Architecture

```
                         ┌─────────────────────┐
                         │   postgres-service    │
                         │  (PostgreSQL 15,      │
                         │   ClusterIP :5433)     │
                         └───────────┬───────────┘
                                     │
              ┌──────────────────────┼──────────────────────┐
              │                      │                       │
   ┌──────────▼──────────┐   ┌───────▼────────┐   ┌─────────▼─────────┐
   │   db-migration        │   │   task-api       │   │  task-worker pod    │
   │  (init container,     │   │  (Gin REST API,  │   │  ┌───────────────┐  │
   │   runs schema          │   │   ClusterIP :80) │   │  │ worker         │  │
   │   migration, exits)    │   │                  │   │  │ (polls task-api)│  │
   └────────────────────────┘   └────────┬─────────┘   │  └──────┬────────┘  │
                                          │              │         │ writes    │
                                          │              │   shared emptyDir   │
                                          │              │   volume: audit.log │
                                          │              │         │           │
                                          │              │  ┌──────▼────────┐  │
                                          └──────────────┼──│ sidecar        │  │
                                             HTTP GET     │  │ (tails log →   │  │
                                             /task/get    │  │  stdout)       │  │
                                                          │  └───────────────┘  │
                                                          └─────────────────────┘
```

## Components

| Component | Description |
|---|---|
| **`task-api/`** | Go + [Gin](https://github.com/gin-gonic/gin) REST API. Exposes `POST /task/add` and `GET /task/get`, backed by PostgreSQL. |
| **`db-migration/`** | Standalone Go binary that connects to Postgres (with retry/backoff) and runs the `tasks` table schema migration. Runs as a Kubernetes **init container** before `task-api` starts. |
| **`worker/`** | Go daemon that polls `task-api` every few seconds for `Pending` tasks, "processes" them, and writes structured audit log entries to a shared log file. |
| **`sidecar-app/`** | Go **sidecar** container that tails the worker's audit log file (from a volume shared with `worker`) and streams new lines to stdout, so logs are visible via `kubectl logs`. |
| **`k8s/`** | Kubernetes manifests for all the above: Deployments, Services, ConfigMap, Secret. |

## Tech Stack

- **Language:** Go 1.25
- **Web framework:** [Gin](https://github.com/gin-gonic/gin)
- **Database:** PostgreSQL 15 (via `lib/pq`)
- **Containerization:** Docker (multi-stage builds, `golang:1.25.1` → `alpine:latest`)
- **Orchestration:** Kubernetes (Deployments, Services, ConfigMaps, Secrets, `emptyDir` volumes, init containers)

## Project Structure

```
.
├── task-api/               # REST API service
│   ├── main.go              # Gin routes & handlers
│   ├── db.go                 # DB connection + repository layer
│   └── task-api.Dockerfile
├── db-migration/           # One-shot schema migration job (init container)
│   ├── main.go
│   └── db-migration.Dockerfile
├── worker/                 # Background task poller
│   ├── main.go
│   └── worker.Dockerfile
├── sidecar-app/            # Log-tailing sidecar
│   ├── main.go
│   └── sidecar.Dockerfile
└── k8s/                    # Kubernetes manifests
    ├── namespace.yaml
    ├── postgres-cred.yml           # ConfigMap + Secret
    ├── postgres-deployment.yml
    ├── postgres-service.yml
    ├── task-api-deployment.yaml     # includes db-migration init container
    ├── task-api-service.yaml
    └── worker-deployment.yml        # worker + sidecar, sharing a volume
```

## How It Works

1. **`postgres-deployment`** spins up a PostgreSQL pod, configured via the `app-config` ConfigMap and `db-secret` Secret.
2. **`task-api-deployment`** first runs the **`db-migration` init container**, which connects to Postgres (retrying until it's ready) and creates the `tasks` table if it doesn't exist. Once the init container completes, the `task-api` container starts and serves HTTP on port `8080` (exposed internally via `task-api-service` on port `80`).
3. **`task-worker-deployment`** runs two containers in the same pod, sharing an `emptyDir` volume mounted at `/app/logs`:
   - `worker` polls `task-api-service` for pending tasks and appends structured entries to `logs/audit.log`.
   - `sidecar` tails that same file and streams new lines to stdout — the classic **sidecar log-streaming pattern**, so `kubectl logs <pod> -c sidecar` shows live activity without the worker needing to know about logging infrastructure.

## Prerequisites

- Docker
- A local Kubernetes cluster (e.g. Minikube, Kind, or Docker Desktop's Kubernetes)
- `kubectl`

## Getting Started

> Images are currently referenced with `imagePullPolicy: Never`, so they must be built locally and available to your cluster's Docker daemon (e.g. `eval $(minikube docker-env)` if using Minikube).

```bash
# 1. Build each service's image
docker build -t task-api:latest -f task-api/task-api.Dockerfile task-api/
docker build -t db-migration:latest -f db-migration/db-migration.Dockerfile db-migration/
docker build -t worker:latest -f worker/worker.Dockerfile worker/
docker build -t sidecar:latest -f sidecar-app/sidecar.Dockerfile sidecar-app/

# 2. Apply the Kubernetes manifests
kubectl apply -f k8s/postgres-cred.yml
kubectl apply -f k8s/postgres-deployment.yml
kubectl apply -f k8s/postgres-service.yml
kubectl apply -f k8s/task-api-deployment.yaml
kubectl apply -f k8s/task-api-service.yaml
kubectl apply -f k8s/worker-deployment.yml

# 3. Check everything is running
kubectl get pods
kubectl logs <task-worker-pod> -c sidecar   # tail the audit log stream
```

## Roadmap

This is an evolving learning project. Planned additions include:

- [ ] Enable and wire up the `task-system` namespace (currently commented out in `k8s/namespace.yaml`)
- [ ] Resource requests/limits and readiness/liveness probes
- [ ] Horizontal Pod Autoscaling for `task-api` and `worker`
- [ ] Ingress for external access to `task-api`
- [ ] Move secrets to a proper secret manager instead of plaintext `Secret` manifests
- [ ] Helm chart for templated deployments
- [ ] CI/CD pipeline for image builds

## Author

**Navneet Shukla** — [@navneetshukl](https://github.com/navneetshukl)
