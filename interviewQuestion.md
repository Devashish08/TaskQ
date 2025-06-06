# TaskQ Interview Questions & Answers

A comprehensive guide to interview questions about the TaskQ job queue system, covering technical architecture, implementation details, and behavioral aspects.

## Table of Contents

1. [System Design & Architecture](#system-design--architecture)
2. [Technical Implementation](#technical-implementation)
3. [Database & Persistence](#database--persistence)
4. [Queue & Redis](#queue--redis)
5. [Concurrency & Performance](#concurrency--performance)
6. [Error Handling & Reliability](#error-handling--reliability)
7. [API Design](#api-design)
8. [Testing & Quality](#testing--quality)
9. [Deployment & Operations](#deployment--operations)
10. [Behavioral Questions](#behavioral-questions)

---

## System Design & Architecture

### Q1: Can you explain the high-level architecture of TaskQ?

**Answer:**
TaskQ follows a clean, layered architecture with clear separation of concerns:

1. **API Layer** (Gin Framework):
   - RESTful endpoints for job submission and status queries
   - Request validation and response formatting
   - HTTP middleware for logging, error handling

2. **Service Layer**:
   - Business logic for job management
   - Orchestrates between API, database, and queue
   - Handles job state transitions

3. **Queue Layer** (Redis):
   - Uses Redis Lists with LPUSH/BRPOP for reliable queuing
   - Provides fast, in-memory job distribution
   - Acts as a buffer between producers and consumers

4. **Worker Pool**:
   - Configurable number of concurrent workers
   - Each worker independently polls Redis
   - Executes jobs based on type-specific handlers

5. **Persistence Layer** (PostgreSQL):
   - Stores complete job history and metadata
   - Enables job status tracking and retry logic
   - Provides durability and audit trail

The flow is: API → Service → Database (persist) → Redis (queue) → Workers → Database (update)

### Q2: Why did you choose this particular tech stack?

**Answer:**
Each technology was chosen for specific reasons:

- **Go**: 
  - Excellent concurrency primitives (goroutines, channels)
  - High performance with low memory footprint
  - Strong standard library for building web services
  - Static typing catches errors at compile time

- **Gin Framework**:
  - Lightweight and fast HTTP router
  - Built-in JSON validation and binding
  - Middleware support for cross-cutting concerns
  - Large ecosystem and community

- **PostgreSQL**:
  - ACID compliance for data integrity
  - JSONB support for flexible payload storage
  - Mature, battle-tested database
  - Rich querying capabilities for job analytics

- **Redis**:
  - Extremely fast in-memory operations
  - Built-in list operations perfect for queuing
  - Atomic operations prevent race conditions
  - Can be persisted for durability if needed

- **Docker**:
  - Consistent development environment
  - Easy dependency management
  - Simplified deployment process

### Q3: How would you scale TaskQ to handle millions of jobs per day?

**Answer:**
Scaling strategy would involve multiple approaches:

1. **Horizontal Scaling**:
   - Deploy multiple API server instances behind a load balancer
   - Increase worker pool size across multiple machines
   - Use Redis Cluster for queue distribution

2. **Database Optimization**:
   - Implement database partitioning by date or job status
   - Add read replicas for status queries
   - Create appropriate indexes (status, created_at, type)
   - Archive completed jobs to separate storage

3. **Queue Optimization**:
   - Implement priority queues using Redis Sorted Sets
   - Multiple queues for different job types/priorities
   - Consider Redis Streams for better scalability

4. **Caching Strategy**:
   - Cache frequently queried job statuses
   - Implement result caching for idempotent jobs
   - Use Redis for session/temporary data

5. **Monitoring & Optimization**:
   - Add metrics collection (Prometheus)
   - Implement distributed tracing
   - Auto-scaling based on queue depth

---

## Technical Implementation

### Q4: Walk me through the job submission flow in detail.

**Answer:**
The job submission flow involves several steps:

1. **API Request Reception**:
   ```go
   POST /jobs
   {
     "type": "email",
     "payload": {"to": "user@example.com", ...}
   }
   ```

2. **Request Validation**:
   - Gin validates JSON structure
   - Check job type is supported
   - Validate payload based on job type

3. **Job Creation**:
   - Generate UUID for job ID
   - Create job record with initial status 'pending'
   - Set timestamps and attempt counter to 0

4. **Database Transaction**:
   ```sql
   INSERT INTO jobs (id, type, payload, status, attempts, created_at, updated_at)
   VALUES ($1, $2, $3, 'pending', 0, NOW(), NOW())
   ```

5. **Queue Push**:
   ```go
   // After successful DB insert
   redisClient.LPush(ctx, "taskq:pending_jobs", jobID)
   ```

6. **Response**:
   - Return job ID and status to client
   - 201 Created status code

7. **Error Handling**:
   - Rollback DB transaction on Redis failure
   - Return appropriate error response

### Q5: How does the worker pool implementation work?

**Answer:**
The worker pool implementation uses Go's concurrency features:

```go
type WorkerPool struct {
    workers    int
    jobHandler JobHandler
    db         *pgx.Pool
    redis      *redis.Client
    wg         sync.WaitGroup
    ctx        context.Context
    cancel     context.CancelFunc
}

func (wp *WorkerPool) Start() {
    for i := 0; i < wp.workers; i++ {
        wp.wg.Add(1)
        go wp.worker(i)
    }
}

func (wp *WorkerPool) worker(id int) {
    defer wp.wg.Done()
    
    for {
        select {
        case <-wp.ctx.Done():
            return
        default:
            // Blocking pop from Redis
            jobID, err := wp.redis.BRPop(wp.ctx, 0, "taskq:pending_jobs").Result()
            if err != nil {
                continue
            }
            
            wp.processJob(jobID[1])
        }
    }
}
```

Key features:
- Each worker runs in its own goroutine
- Workers use `BRPOP` for blocking Redis operations
- Context-based cancellation for graceful shutdown
- WaitGroup ensures all workers finish before exit
- Independent workers prevent cascading failures

### Q6: Explain the job retry mechanism implementation.

**Answer:**
The retry mechanism is implemented with exponential backoff:

```go
func (wp *WorkerPool) processJob(jobID string) {
    // 1. Fetch job from database
    job, err := wp.fetchJob(jobID)
    if err != nil {
        return
    }
    
    // 2. Update status to 'running'
    wp.updateJobStatus(jobID, "running")
    
    // 3. Execute job handler
    handler, exists := wp.jobHandler.GetHandler(job.Type)
    if !exists {
        wp.failJob(jobID, "Unknown job type")
        return
    }
    
    err = handler(job.Payload)
    
    if err == nil {
        // 4a. Success - mark as completed
        wp.updateJobStatus(jobID, "completed")
    } else {
        // 4b. Failure - check retry logic
        job.Attempts++
        
        if job.Attempts < MaxRetries {
            // Update attempts and re-queue
            wp.updateJobAttempts(jobID, job.Attempts)
            wp.updateJobStatus(jobID, "pending")
            
            // Re-queue with delay (could use Redis ZADD for delayed queue)
            time.Sleep(time.Duration(job.Attempts) * time.Second)
            wp.redis.LPush(ctx, "taskq:pending_jobs", jobID)
        } else {
            // Max retries reached
            wp.failJob(jobID, err.Error())
        }
    }
}
```

Features:
- Configurable maximum retry attempts
- Exponential backoff between retries
- Error messages stored for debugging
- Failed jobs remain in database for analysis

---

## Database & Persistence

### Q7: What's your database schema design and why?

**Answer:**
The database schema is designed for flexibility and performance:

```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    
    -- Indexes for common queries
    INDEX idx_jobs_status (status),
    INDEX idx_jobs_created_at (created_at),
    INDEX idx_jobs_type_status (type, status)
);
```

Design decisions:
- **UUID primary key**: Avoids ID conflicts in distributed systems
- **JSONB payload**: Flexible schema for different job types
- **Status as string**: Human-readable, extensible
- **Timestamps**: Enable time-based queries and TTL
- **Indexes**: Optimize common query patterns

### Q8: How do you handle database connection pooling?

**Answer:**
Connection pooling is managed using pgx's built-in pool:

```go
func NewDatabase(cfg *config.Config) (*pgx.Pool, error) {
    poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
    if err != nil {
        return nil, err
    }
    
    // Configure pool settings
    poolConfig.MaxConns = 25
    poolConfig.MinConns = 5
    poolConfig.MaxConnLifetime = time.Hour
    poolConfig.MaxConnIdleTime = time.Minute * 30
    
    // Health check
    poolConfig.HealthCheckPeriod = time.Minute
    
    pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
    if err != nil {
        return nil, err
    }
    
    return pool, nil
}
```

Benefits:
- Reuses connections to reduce overhead
- Configurable pool size based on load
- Automatic health checks and reconnection
- Context-based query cancellation

---

## Queue & Redis

### Q9: Why use Redis Lists instead of other Redis data structures?

**Answer:**
Redis Lists were chosen for specific reasons:

**Advantages of Lists**:
1. **FIFO Queue**: Natural queue semantics with LPUSH/RPOP
2. **Blocking Operations**: BRPOP eliminates polling overhead
3. **Atomicity**: Operations are atomic, preventing race conditions
4. **Simplicity**: Easy to implement and understand

**Comparison with alternatives**:
- **Sorted Sets**: More complex, better for priority queues
- **Streams**: Better for event sourcing, overkill for simple queue
- **Pub/Sub**: No persistence, lost messages on disconnect

**Current implementation**:
```go
// Producer
redis.LPush(ctx, "taskq:pending_jobs", jobID)

// Consumer (blocking)
result, err := redis.BRPop(ctx, 0, "taskq:pending_jobs").Result()
```

### Q10: How do you handle the Redis queue reliability problem?

**Answer:**
The current implementation has a known reliability issue - jobs can be lost if a worker crashes after BRPOP but before job completion. Solutions:

1. **Current Mitigation**:
   - Jobs are persisted in PostgreSQL first
   - Can implement a "job recovery" process for orphaned jobs
   
2. **Reliable Queue Pattern** (Future enhancement):
   ```go
   // Use RPOPLPUSH for reliability
   jobID, err := redis.RPopLPush(ctx, "pending", "processing").Result()
   
   // After job completion
   redis.LRem(ctx, "processing", 1, jobID)
   
   // Periodic check for stuck jobs
   processingJobs := redis.LRange(ctx, "processing", 0, -1).Val()
   for _, jobID := range processingJobs {
       if isStuck(jobID) {
           redis.LPush(ctx, "pending", jobID)
       }
   }
   ```

3. **Alternative Solutions**:
   - Use Redis Streams with consumer groups
   - Implement acknowledgment pattern
   - Consider dedicated message queue (RabbitMQ, NATS)

---

## Concurrency & Performance

### Q11: How do you prevent race conditions in your system?

**Answer:**
Multiple strategies prevent race conditions:

1. **Database Level**:
   - Row-level locking during updates
   - Transactions for atomic operations
   ```sql
   BEGIN;
   SELECT * FROM jobs WHERE id = $1 FOR UPDATE;
   UPDATE jobs SET status = $2 WHERE id = $1;
   COMMIT;
   ```

2. **Redis Level**:
   - Atomic operations (LPUSH, BRPOP)
   - Single-threaded nature of Redis
   - No shared state between workers

3. **Application Level**:
   - Each worker operates independently
   - No shared memory between goroutines
   - Context-based cancellation

4. **Idempotency**:
   - Job handlers designed to be idempotent
   - Status checks before processing

### Q12: What performance optimizations have you implemented?

**Answer:**
Several optimizations improve performance:

1. **Database Optimizations**:
   ```sql
   -- Partial index for active jobs
   CREATE INDEX idx_active_jobs ON jobs(created_at) 
   WHERE status IN ('pending', 'running');
   
   -- Prepared statements
   stmt, err := db.Prepare("UPDATE jobs SET status = $1 WHERE id = $2")
   ```

2. **Batch Operations**:
   ```go
   // Batch fetch jobs for status endpoint
   func GetJobsByStatus(status string, limit int) ([]Job, error) {
       query := `SELECT id, type, status, created_at 
                 FROM jobs 
                 WHERE status = $1 
                 ORDER BY created_at DESC 
                 LIMIT $2`
       // Use single query instead of N+1
   }
   ```

3. **Connection Reuse**:
   - Database connection pooling
   - Redis connection persistence
   - HTTP keep-alive for API

4. **Goroutine Management**:
   - Worker pool pattern prevents goroutine explosion
   - Buffered channels where appropriate

---

## Error Handling & Reliability

### Q13: How does your error handling strategy work?

**Answer:**
Comprehensive error handling at multiple levels:

1. **API Level**:
   ```go
   func HandleError(c *gin.Context, err error) {
       var apiErr *APIError
       if errors.As(err, &apiErr) {
           c.JSON(apiErr.StatusCode, gin.H{
               "error": apiErr.Message,
               "code": apiErr.Code,
           })
           return
       }
       
       // Log internal errors, return generic message
       log.Error("Internal error", "error", err)
       c.JSON(500, gin.H{
           "error": "Internal server error",
           "code": "INTERNAL_ERROR",
       })
   }
   ```

2. **Worker Level**:
   - Graceful error recovery
   - Error logging with context
   - Job-specific error handling

3. **Database Level**:
   - Connection retry logic
   - Transaction rollback
   - Deadlock detection

4. **Monitoring**:
   - Structured logging
   - Error rate metrics
   - Alert thresholds

### Q14: How do you ensure data consistency?

**Answer:**
Data consistency is maintained through:

1. **Transaction Management**:
   ```go
   tx, err := db.Begin(ctx)
   defer tx.Rollback(ctx)
   
   // Insert job
   err = tx.QueryRow(ctx, insertJob, ...).Scan(&jobID)
   
   // Only queue if DB insert succeeds
   err = redis.LPush(ctx, "pending", jobID).Err()
   if err != nil {
       return err // Rollback happens
   }
   
   tx.Commit(ctx)
   ```

2. **State Machine**:
   - Valid state transitions only
   - No backward transitions (except retry)
   - Atomic state updates

3. **Eventual Consistency**:
   - Job recovery process for orphaned jobs
   - Periodic consistency checks
   - Reconciliation procedures

---

## API Design

### Q15: How did you design your REST API?

**Answer:**
The API follows RESTful principles:

1. **Resource-Oriented**:
   - Jobs are the primary resource
   - Clear URL structure: `/jobs`, `/jobs/{id}`
   
2. **HTTP Methods**:
   - POST /jobs - Create new job
   - GET /jobs/{id} - Get job status
   - GET /jobs?status=failed - List jobs by status (future)

3. **Status Codes**:
   - 201 Created - Job successfully queued
   - 200 OK - Status retrieved
   - 404 Not Found - Job doesn't exist
   - 400 Bad Request - Invalid input
   - 500 Internal Error - Server issues

4. **Response Format**:
   ```json
   {
     "id": "uuid",
     "status": "pending|running|completed|failed",
     "type": "email",
     "created_at": "2023-12-01T10:00:00Z",
     "error": "error message if failed"
   }
   ```

5. **Versioning Strategy**:
   - URL versioning: `/v1/jobs`
   - Backward compatibility commitment

### Q16: How would you implement API rate limiting?

**Answer:**
Rate limiting implementation strategy:

```go
// Using Redis for distributed rate limiting
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
    return func(c *gin.Context) {
        clientIP := c.ClientIP()
        key := fmt.Sprintf("rate_limit:%s", clientIP)
        
        // Increment counter
        count, err := redis.Incr(ctx, key).Result()
        if err != nil {
            c.AbortWithStatus(500)
            return
        }
        
        // Set expiry on first request
        if count == 1 {
            redis.Expire(ctx, key, window)
        }
        
        // Check limit
        if count > int64(limit) {
            c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
            c.Header("X-RateLimit-Remaining", "0")
            c.AbortWithStatusJSON(429, gin.H{
                "error": "Rate limit exceeded",
            })
            return
        }
        
        c.Header("X-RateLimit-Remaining", strconv.Itoa(limit-int(count)))
        c.Next()
    }
}
```

Additional considerations:
- Token bucket algorithm for smoother limiting
- Different limits per endpoint
- API key-based limits for authenticated users

---

## Testing & Quality

### Q17: What's your testing strategy for TaskQ?

**Answer:**
Comprehensive testing approach:

1. **Unit Tests**:
   ```go
   func TestJobHandler_Execute(t *testing.T) {
       handler := NewEmailHandler(mockSMTP)
       payload := json.RawMessage(`{"to":"test@example.com"}`)
       
       err := handler.Execute(payload)
       assert.NoError(t, err)
       assert.Equal(t, 1, mockSMTP.CallCount())
   }
   ```

2. **Integration Tests**:
   ```go
   func TestJobSubmissionFlow(t *testing.T) {
       // Setup test database and Redis
       db := setupTestDB(t)
       redis := setupTestRedis(t)
       
       // Submit job via API
       resp := httptest.NewRecorder()
       req := httptest.NewRequest("POST", "/jobs", jobJSON)
       router.ServeHTTP(resp, req)
       
       assert.Equal(t, 201, resp.Code)
       
       // Verify in database
       var job Job
       err := db.Get(&job, "SELECT * FROM jobs WHERE id = $1", jobID)
       assert.NoError(t, err)
       assert.Equal(t, "pending", job.Status)
       
       // Verify in Redis
       exists := redis.Exists(ctx, "pending_jobs").Val()
       assert.Equal(t, int64(1), exists)
   }
   ```

3. **End-to-End Tests**:
   - Docker Compose test environment
   - Full workflow testing
   - Performance benchmarks

4. **Test Coverage**:
   - Aim for >80% coverage
   - Focus on critical paths
   - Mock external dependencies

### Q18: How do you ensure code quality?

**Answer:**
Multiple quality assurance measures:

1. **Static Analysis**:
   ```yaml
   # .golangci.yml configuration
   linters:
     enable:
       - errcheck
       - govet
       - ineffassign
       - staticcheck
       - unused
   ```

2. **Code Review Process**:
   - PR-based workflow
   - At least one reviewer
   - Automated checks must pass

3. **Documentation**:
   - Code comments for complex logic
   - API documentation
   - Architecture decisions recorded

4. **Performance Testing**:
   ```go
   func BenchmarkJobProcessing(b *testing.B) {
       setup()
       b.ResetTimer()
       
       for i := 0; i < b.N; i++ {
           processJob(testJob)
       }
   }
   ```

---

## Deployment & Operations

### Q19: How would you deploy TaskQ to production?

**Answer:**
Production deployment strategy:

1. **Containerization**:
   ```dockerfile
   # Multi-stage build
   FROM golang:1.22 AS builder
   WORKDIR /app
   COPY go.mod go.sum ./
   RUN go mod download
   COPY . .
   RUN CGO_ENABLED=0 go build -o taskq ./cmd/taskq-server

   FROM alpine:3.18
   RUN apk --no-cache add ca-certificates
   COPY --from=builder /app/taskq /taskq
   ENTRYPOINT ["/taskq"]
   ```

2. **Kubernetes Deployment**:
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: taskq-api
   spec:
     replicas: 3
     template:
       spec:
         containers:
         - name: taskq
           image: taskq:latest
           env:
           - name: DB_HOST
             valueFrom:
               secretKeyRef:
                 name: taskq-secrets
                 key: db-host
   ```

3. **Infrastructure**:
   - Managed PostgreSQL (AWS RDS)
   - Redis cluster (AWS ElastiCache)
   - Application on EKS/GKE
   - Load balancer for API

4. **CI/CD Pipeline**:
   - GitHub Actions for testing
   - Docker image building
   - Rolling deployments
   - Health checks

### Q20: What monitoring and observability would you add?

**Answer:**
Comprehensive monitoring strategy:

1. **Metrics (Prometheus)**:
   ```go
   var (
       jobsCreated = prometheus.NewCounterVec(
           prometheus.CounterOpts{
               Name: "taskq_jobs_created_total",
               Help: "Total number of jobs created",
           },
           []string{"type"},
       )
       
       jobDuration = prometheus.NewHistogramVec(
           prometheus.HistogramOpts{
               Name: "taskq_job_duration_seconds",
               Help: "Job processing duration",
           },
           []string{"type", "status"},
       )
   )
   ```

2. **Logging (Structured)**:
   ```go
   logger.Info("Job processed",
       "job_id", jobID,
       "type", job.Type,
       "duration", duration,
       "status", status,
   )
   ```

3. **Tracing (OpenTelemetry)**:
   - Distributed tracing across services
   - Performance bottleneck identification
   - Request flow visualization

4. **Alerts**:
   - High error rate
   - Queue depth threshold
   - Worker pool exhaustion
   - Database connection issues

---

## Behavioral Questions

### Q21: What was the most challenging part of building TaskQ?

**Answer:**
The most challenging aspect was designing the reliable queue mechanism. The initial implementation using simple Redis Lists had a critical flaw - jobs could be lost if a worker crashed after popping from the queue but before completing the job.

I researched various patterns:
1. Redis reliable queue pattern using RPOPLPUSH
2. Two-phase commit approaches
3. Alternative queue technologies

The challenge taught me:
- Always consider failure modes in distributed systems
- The importance of understanding tool limitations
- Trade-offs between simplicity and reliability
- Value of incremental improvements

### Q22: How did you approach the project planning?

**Answer:**
I followed an iterative, phase-based approach:

1. **Phase 1 - MVP**: 
   - Started with the simplest possible implementation
   - In-memory queue to validate the concept
   - Focus on core flow: receive job → process job

2. **Phase 2 - Persistence**:
   - Added real infrastructure (PostgreSQL, Redis)
   - Ensured data durability
   - Maintained backward compatibility

3. **Phase 3 - Robustness**:
   - Added error handling, retries
   - Multiple workers for scalability
   - Comprehensive testing

4. **Phase 4 - Polish** (planned):
   - Production-ready features
   - Monitoring and observability
   - Performance optimizations

This approach allowed:
- Early validation of concepts
- Continuous improvement
- Clear milestones
- Flexibility to adapt

### Q23: How do you handle technical debt in this project?

**Answer:**
Technical debt is managed through:

1. **Documentation**:
   - TODO comments for known issues
   - README section for limitations
   - Architecture decision records

2. **Prioritization**:
   - Critical: Data loss risks (queue reliability)
   - High: Performance bottlenecks
   - Medium: Code refactoring
   - Low: Nice-to-have features

3. **Regular Refactoring**:
   - Allocate time in each phase
   - Refactor before adding features
   - Keep changes small and tested

Example of addressed debt:
- Initially, all code was in main.go
- Refactored into clean architecture
- Added proper error handling
- Improved configuration management

### Q24: How would you onboard a new developer to this project?

**Answer:**
Comprehensive onboarding process:

1. **Documentation Review**:
   - Start with README for overview
   - Review architecture diagrams
   - Understand the problem domain

2. **Local Setup**:
   - Follow quick start guide
   - Run make commands
   - Submit a test job

3. **Code Walkthrough**:
   - Start with main.go entry point
   - Follow job submission flow
   - Understand worker implementation

4. **First Tasks**:
   - Add a new job type (well-defined scope)
   - Write tests for existing code
   - Fix a documented bug

5. **Pair Programming**:
   - Work together on complex features
   - Share context and decisions
   - Review code together

### Q25: What would you do differently if starting over?

**Answer:**
With hindsight, I would make several changes:

1. **Start with Reliable Queue**:
   - Implement RPOPLPUSH pattern from day one
   - Or use Redis Streams for built-in reliability
   - Avoid the technical debt of unreliable queue

2. **Better Abstraction**:
   - Queue interface from the beginning
   - Easier to swap implementations
   - Better testing with mocks

3. **Observability First**:
   - Add metrics/logging from start
   - Easier debugging during development
   - Performance baseline data

4. **More Comprehensive Testing**:
   - TDD approach for critical paths
   - Integration tests earlier
   - Load testing sooner

5. **Configuration Management**:
   - Use Viper or similar from start
   - Environment-based configs
   - Feature flags for gradual rollout

However, the iterative approach was valuable for learning and understanding the problem space progressively.

---

## Advanced Technical Questions

### Q26: How would you implement priority queues?

**Answer:**
Priority queue implementation using Redis Sorted Sets:

```go
// Job submission with priority
type JobRequest struct {
    Type     string          `json:"type"`
    Payload  json.RawMessage `json:"payload"`
    Priority int             `json:"priority"` // 0-9, higher = more important
}

// Queue implementation
func QueueJobWithPriority(jobID string, priority int) error {
    // Score = current timestamp - priority * 1000
    // This ensures FIFO within same priority
    score := float64(time.Now().Unix()) - float64(priority*1000)
    
    return redis.ZAdd(ctx, "taskq:priority_queue", &redis.Z{
        Score:  score,
        Member: jobID,
    }).Err()
}

// Worker modification
func (w *Worker) fetchNextJob() (string, error) {
    // Pop highest priority (lowest score)
    results, err := redis.ZPopMin(ctx, "taskq:priority_queue", 1).Result()
    if err != nil || len(results) == 0 {
        return "", err
    }
    
    return results[0].Member.(string), nil
}
```

### Q27: How would you handle distributed transactions?

**Answer:**
Distributed transaction handling using Saga pattern:

```go
type JobSaga struct {
    steps []SagaStep
    compensations []CompensationFunc
}

type SagaStep func(ctx context.Context, job *Job) error
type CompensationFunc func(ctx context.Context, job *Job) error

func (s *JobSaga) Execute(ctx context.Context, job *Job) error {
    completed := 0
    
    for i, step := range s.steps {
        if err := step(ctx, job); err != nil {
            // Rollback in reverse order
            for j := completed - 1; j >= 0; j-- {
                s.compensations[j](ctx, job)
            }
            return err
        }
        completed = i + 1
    }
    
    return nil
}

// Example usage for distributed job
saga := &JobSaga{
    steps: []SagaStep{
        createDatabaseRecord,
        reserveResourceQuota,
        queueForProcessing,
    },
    compensations: []CompensationFunc{
        deleteDatabaseRecord,
        releaseResourceQuota,
        removeFromQueue,
    },
}
```

### Q28: How would you implement job dependencies?

**Answer:**
Job dependency management through DAG (Directed Acyclic Graph):

```go
type JobDependency struct {
    JobID      string   `json:"job_id"`
    DependsOn  []string `json:"depends_on"`
    Status     string   `json:"status"`
}

func SubmitJobWithDependencies(job Job, dependencies []string) error {
    // 1. Validate no circular dependencies
    if hasCycle(job.ID, dependencies) {
        return errors.New("circular dependency detected")
    }
    
    // 2. Store job in pending state
    err := db.StoreJob(job, "waiting_dependencies")
    
    // 3. Store dependency graph
    for _, depID := range dependencies {
        err = db.StoreDependency(job.ID, depID)
    }
    
    // 4. Check if ready to run
    checkAndQueueIfReady(job.ID)
    
    return nil
}

func OnJobComplete(jobID string) {
    // Find dependent jobs
    dependents := db.GetDependentJobs(jobID)
    
    for _, dependent := range dependents {
        if allDependenciesComplete(dependent) {
            queueJob(dependent)
        }
    }
}
```

---

## System Design Extensions

### Q29: How would you design TaskQ for multi-tenancy?

**Answer:**
Multi-tenant architecture considerations:

1. **Data Isolation**:
   ```sql
   -- Add tenant_id to jobs table
   ALTER TABLE jobs ADD COLUMN tenant_id UUID NOT NULL;
   CREATE INDEX idx_jobs_tenant ON jobs(tenant_id, status);
   
   -- Row-level security
   CREATE POLICY tenant_isolation ON jobs
   FOR ALL USING (tenant_id = current_setting('app.current_tenant')::uuid);
   ```

2. **Queue Isolation**:
   ```go
   // Separate queues per tenant
   queueKey := fmt.Sprintf("taskq:%s:pending_jobs", tenantID)
   redis.LPush(ctx, queueKey, jobID)
   ```

3. **Resource Limits**:
   ```go
   type TenantLimits struct {
       MaxConcurrentJobs int
       MaxJobsPerDay     int
       MaxJobSize        int64
   }
   
   func CheckTenantLimits(tenantID string) error {
       current := GetTenantUsage(tenantID)
       limits := GetTenantLimits(tenantID)
       
       if current.ConcurrentJobs >= limits.MaxConcurrentJobs {
           return ErrRateLimitExceeded
       }
       // Additional checks...
   }
   ```

4. **Fair Scheduling**:
   - Round-robin between tenant queues
   - Priority based on SLA tiers
   - Resource quota enforcement

### Q30: How would you make TaskQ cloud-native?

**Answer:**
Cloud-native transformation:

1. **12-Factor App Principles**:
   - Config through environment variables
   - Stateless workers
   - Port binding
   - Disposability

2. **Kubernetes Native**:
   ```yaml
   # HorizontalPodAutoscaler
   apiVersion: autoscaling/v2
   kind: HorizontalPodAutoscaler
   metadata:
     name: taskq-workers
   spec:
     scaleTargetRef:
       apiVersion: apps/v1
       kind: Deployment
       name: taskq-workers
     minReplicas: 2
     maxReplicas: 50
     metrics:
     - type: Pods
       pods:
         metric:
           name: redis_queue_depth
         target:
           type: AverageValue
           averageValue: "30"
   ```

3. **Service Mesh Integration**:
   - Istio for traffic management
   - mTLS for service communication
   - Circuit breakers

4. **Cloud Services Integration**:
   - AWS SQS/Google Pub/Sub as queue
   - Managed databases
   - Object storage for large payloads
   - Serverless workers (Lambda/Cloud Functions)

---

## Conclusion

These questions and answers cover the complete spectrum of technical and behavioral aspects of the TaskQ project. They demonstrate:

- **Technical Depth**: Understanding of distributed systems, concurrency, and reliability
- **Problem-Solving**: Approach to challenges and trade-offs
- **Code Quality**: Testing, monitoring, and maintenance considerations
- **Growth Mindset**: Learning from mistakes and iterative improvement
- **Communication**: Ability to explain complex technical concepts clearly

Remember to tailor your responses based on the specific role and company you're interviewing for, emphasizing the aspects most relevant to their needs. 