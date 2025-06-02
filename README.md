# TaskQ: Golang Job Queue Backend

[![Go Version](https://img.shields.io/badge/Go-1.22.2-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

TaskQ is a robust backend service designed to accept tasks via an API, queue them reliably, and process them asynchronously using background workers. This allows primary application services to offload long-running or deferrable tasks, improving responsiveness and system resilience.

## ✨ Features

- **RESTful API** for job submission and status tracking
- **Asynchronous job processing** with configurable worker pools
- **Reliable job persistence** using PostgreSQL
- **Redis-based queuing** for fast job distribution
- **Retry mechanism** for failed jobs with configurable attempts
- **Multiple job types** support with pluggable job handlers
- **Docker-based development** environment
- **Graceful error handling** and status tracking

## 🏗️ Architecture

TaskQ follows a clean architecture pattern with the following components:

- **API Layer**: HTTP endpoints for job submission and status queries
- **Service Layer**: Business logic for job management
- **Queue Layer**: Redis-based job distribution
- **Worker Pool**: Concurrent job processors
- **Persistence Layer**: PostgreSQL for job state and history

## 🛠️ Tech Stack

- **Language**: Go 1.22.2
- **Web Framework**: Gin
- **Database**: PostgreSQL 15
- **Cache/Queue**: Redis 7
- **Containerization**: Docker & Docker Compose
- **Database Driver**: pgx/v5
- **Redis Client**: go-redis/v9

## 📋 Prerequisites

- Docker and Docker Compose
- Go 1.22.2 or later (for development)
- Make (optional, for using Makefile commands)

## 🚀 Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/Devashish08/taskq.git
cd taskq
```

### 2. Start Infrastructure Services

```bash
make docker-up
```

This will start PostgreSQL and Redis containers with the following default configurations:
- **PostgreSQL**: `localhost:5432` (user: `taskq_user`, password: `taskq_password`, db: `taskq_db`)
- **Redis**: `localhost:6379`

### 3. Run the Application

```bash
make run
```

The API server will start on `http://localhost:8080`

## 📖 API Documentation

### Submit a Job

```bash
POST /jobs
Content-Type: application/json

{
  "type": "email",
  "payload": {
    "to": "user@example.com",
    "subject": "Welcome!",
    "body": "Welcome to our service!"
  }
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```

### Get Job Status

```bash
GET /jobs/{job_id}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "email",
  "status": "completed",
  "payload": {...},
  "created_at": "2023-12-01T10:00:00Z",
  "updated_at": "2023-12-01T10:00:05Z",
  "attempts": 1
}
```

### Job Statuses

- `pending`: Job is queued and waiting to be processed
- `running`: Job is currently being processed by a worker
- `completed`: Job has been successfully completed
- `failed`: Job has failed after all retry attempts

## 🔧 Development

### Available Make Commands

```bash
make help                 # Show all available commands
make run                  # Run the application with development settings
make build                # Build the application binary
make test                 # Run tests
make lint                 # Run golangci-lint
make fmt                  # Format code
make docker-up            # Start Docker services (PostgreSQL & Redis)
make docker-down          # Stop Docker services
make docker-ps            # Show Docker services status
make psql                 # Connect to PostgreSQL database
make redis-cli            # Connect to Redis CLI
```

### Project Structure

```
TaskQ/
├── cmd/
│   └── taskq-server/          # Application entry point
├── internal/
│   ├── api/                   # HTTP handlers and routes
│   ├── config/                # Configuration management
│   ├── database/              # Database connection and queries
│   ├── jobhandlers/           # Job execution logic
│   ├── models/                # Data models
│   ├── redisClient/           # Redis client setup
│   ├── service/               # Business logic
│   └── worker/                # Worker pool implementation
├── migrations/                # Database migration files
├── docker-compose.yml         # Docker services configuration
├── Makefile                   # Development commands
├── go.mod                     # Go module definition
└── README.md                  # This file
```

### Adding New Job Types

1. Create a new job handler function in `internal/jobhandlers/`
2. Register the handler in the job registry
3. The handler should match the signature: `func(payload json.RawMessage) error`

Example:
```go
func HandleEmailJob(payload json.RawMessage) error {
    var emailData EmailPayload
    if err := json.Unmarshal(payload, &emailData); err != nil {
        return err
    }
    
    // Process email sending logic here
    log.Printf("Sending email to: %s", emailData.To)
    
    return nil
}
```

### Environment Variables

The application can be configured using the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `taskq_user` | PostgreSQL username |
| `DB_PASSWORD` | `taskq_password` | PostgreSQL password |
| `DB_NAME` | `taskq_db` | PostgreSQL database name |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_DB` | `0` | Redis database number |
| `WORKER_COUNT` | `3` | Number of worker goroutines |
| `MAX_RETRIES` | `3` | Maximum retry attempts for failed jobs |

## 🧪 Testing

Run the test suite:

```bash
make test
```

For testing with a live database, ensure Docker services are running:

```bash
make docker-up
make test
```

## 🔍 Monitoring

### Database Inspection

Connect to PostgreSQL to inspect jobs:

```bash
make psql
```

```sql
-- View all jobs
SELECT id, type, status, attempts, created_at, updated_at FROM jobs ORDER BY created_at DESC;

-- View failed jobs
SELECT * FROM jobs WHERE status = 'failed';

-- View job statistics
SELECT status, COUNT(*) FROM jobs GROUP BY status;
```

### Redis Queue Inspection

Connect to Redis to inspect the queue:

```bash
make redis-cli
```

```redis
# Check pending jobs count
LLEN taskq:pending_jobs

# View pending job IDs (without removing them)
LRANGE taskq:pending_jobs 0 -1

# Monitor Redis commands in real-time
MONITOR
```

## 🛣️ Roadmap

See [plan.MD](plan.MD) for the detailed development roadmap. Current implementation includes:

- ✅ **Phase 1**: Basic skeleton & in-memory processing
- ✅ **Phase 2**: Persistence & Redis queue
- ✅ **Phase 3**: Robustness, error handling & API enhancements
- 🔄 **Phase 4**: Advanced features & polish (planned)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and idioms
- Write tests for new functionality
- Run `make lint` and `make fmt` before committing
- Update documentation for API changes

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📧 Support

For questions, issues, or contributions, please:

- Open an issue on GitHub
- Check existing issues before creating new ones
- Provide detailed information for bug reports

---

**TaskQ** - Reliable job processing for modern applications 🚀 