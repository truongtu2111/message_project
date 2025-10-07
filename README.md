# Insider Messaging Service

A Go-based messaging service that processes and sends messages through webhooks with scheduling capabilities.

## Features

- Message processing with webhook delivery
- Configurable batch processing and scheduling
- PostgreSQL database with Redis caching
- REST API with Swagger documentation
- Prometheus metrics and structured logging
- Docker containerization

## Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- PostgreSQL
- Redis (optional)

### Development

```bash
# Clone and setup
git clone <repository-url>
cd insider-messaging

# Install dependencies
go mod download

# Run with Docker Compose
docker-compose up -d

# Build and run locally
make build
make run
```

### API Endpoints

- `GET /healthz` - Health check
- `POST /scheduler/start` - Start message scheduler
- `POST /scheduler/stop` - Stop message scheduler
- `GET /messages/sent` - List sent messages
- `GET /swagger/index.html` - API documentation

## Configuration

Environment variables:

- `DB_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string (optional)
- `WEBHOOK_URL` - Target webhook endpoint
- `INTERVAL` - Scheduler interval (default: 2m)
- `BATCH_SIZE` - Messages per batch (default: 2)
- `AUTOSTART` - Auto-start scheduler (default: false)

## Development

```bash
# Run tests
make test

# Run linter
make lint

# Generate swagger docs
make swagger

# Run with coverage
make cover
```

## Architecture

The service follows clean architecture principles with clear separation of concerns:

- `cmd/server` - Application entry point
- `internal/api` - HTTP handlers and routing
- `internal/service` - Business logic
- `internal/repo` - Data access layer
- `internal/integration` - External service clients
- `pkg/` - Shared utilities

## License

MIT License