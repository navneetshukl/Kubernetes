# Makefile for building Docker images and managing them in minikube

# Image names
API_IMAGE := notification-api:latest
WORKER_IMAGE := notification-worker:latest
SIDECAR_IMAGE:=	notification-sidecar:latest

# Dockerfile paths
API_DOCKERFILE := notification-api/api.dockerfile
WORKER_DOCKERFILE := notification-worker/worker.dockerfile
SIDECAR_DOCKERFILE:= notification-sidecar/sidecar.dockerfile

# Build context directories
API_CONTEXT := notification-api
WORKER_CONTEXT := notification-worker
SIDECAR_CONTEXT:= notification-sidecar

.PHONY: all build clean load push deploy help

# Default target
all: build load

# Build both Docker images
build: build-api build-worker

build-api:
	@echo "Building API image..."
	docker build -f $(API_DOCKERFILE) -t $(API_IMAGE) $(API_CONTEXT)

build-worker:
	@echo "Building Worker image..."
	docker build -f $(WORKER_DOCKERFILE) -t $(WORKER_IMAGE) $(WORKER_CONTEXT)

build-sidecar:
	@echo "Building Sidecar image..."
	docker build -f $(SIDECAR_DOCKERFILE) -t $(SIDECAR_IMAGE) $(SIDECAR_CONTEXT)

# Delete images from minikube
clean:
	@echo "Deleting images from minikube..."
	-minikube image rm $(API_IMAGE) 2>/dev/null || true
	-minikube image rm $(WORKER_IMAGE) 2>/dev/null || true
	-minikube image rm $(SIDECAR_IMAGE) 2>/dev/null || true

# Load images into minikube (equivalent to push for minikube)
load: load-api load-worker

load-api:
	@echo "Loading API image into minikube..."
	minikube image load $(API_IMAGE)

load-worker:
	@echo "Loading Worker image into minikube..."
	minikube image load $(WORKER_IMAGE)

load-sidecar:
	@echo "Loading Sidecar image into minikube..."
	minikube image load $(SIDECAR_IMAGE)

# Alias for load (more intuitive name)
push: load

# Full cycle: clean, build, load
rebuild: clean build load

# Deploy to Kubernetes
deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/

# Restart deployments to pick up new images
restart:
	@echo "Restarting deployments..."
	kubectl rollout restart deployment/notification-api-deployment
	kubectl rollout restart deployment/notification-worker-deployment

# Full rebuild and deploy
redeploy: rebuild deploy restart

# Show status
status:
	@echo "=== Minikube Images ==="
	@minikube image ls | grep -E "notification-api|notification-worker" || echo "No notification images found"
	@echo ""
	@echo "=== Kubernetes Pods (all) ==="
	@kubectl get pods  2>/dev/null || true

remove:
	kubectl delete all --all

# Help
help:
	@echo "Available targets:"
	@echo "  make build      - Build both Docker images"
	@echo "  make build-api  - Build only API image"
	@echo "  make build-worker - Build only Worker image"
	@echo "  make clean      - Delete images from minikube"
	@echo "  make load       - Load images into minikube (alias: push)"
	@echo "  make push       - Same as load"
	@echo "  make rebuild    - Clean, build, and load"
	@echo "  make deploy     - Apply Kubernetes manifests"
	@echo "  make restart    - Restart deployments to pick up new images"
	@echo "  make redeploy   - Full rebuild, deploy, and restart"
	@echo "  make status     - Show image and deployment status"
	@echo "  make help       - Show this help"