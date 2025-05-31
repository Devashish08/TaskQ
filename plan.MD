# TaskQ: Golang Job Queue Backend

## Overview

TaskQ is a backend service designed to accept tasks via an API, queue them reliably, and process them asynchronously using background workers. This allows primary application services to offload long-running or deferrable tasks, improving responsiveness and resilience.

## Core Tech Stack (Initial)

*   **Language:** Golang
*   **API Framework:** Gin (or Echo, choose one)
*   **Database:** PostgreSQL (for job persistence and state tracking)
*   **Queue:** Redis (using Lists for basic queuing initially)
*   **Containerization:** Docker / Docker Compose

## Project Plan & Progress

### Phase 1: Basic Skeleton & In-Memory Processing

**Goal:** Create the bare minimum structure: receive a job via API, pass it to a single worker via an in-memory channel, and have the worker print it. No persistence yet.

*   [✅] **Step 1.1: Project Setup**
    *   Initialize Go module (`go mod init github.com/your-username/taskq`)
    *   Create basic directory structure (`/cmd`, `/internal`, `/api`, `/worker`, `/config`, etc.)
    *   Setup basic linting (e.g., `golangci-lint`) and formatting (`go fmt`).
    *   *Commit Message:* `chore: Initialize project structure and Go modules`
*   [✅] **Step 1.2: Basic API Server**
    *   Implement a simple HTTP server using Gin/Echo.
    *   Create a `POST /jobs` endpoint.
    *   For now, this endpoint just reads the request body (expecting simple JSON like `{"type": "email", "payload": {"to": "test@example.com"}}`) and prints it to the console.
    *   *Commit Message:* `feat: Implement basic HTTP server with POST /jobs endpoint`
*   [✅] **Step 1.3: Job Definition & In-Memory Queue**
    *   Define a Go `struct` representing a Job (e.g., `ID`, `Type`, `Payload`, `Status`).
    *   Create a simple buffered Go channel (`chan Job`) to act as the in-memory queue.
    *   Modify the `POST /jobs` handler to unmarshal the JSON into the `Job` struct and send it to the channel.
    *   *Commit Message:* `feat: Define Job struct and implement in-memory queue using channel`
*   [✅] **Step 1.4: Single Basic Worker**
    *   Create a function that runs in a separate goroutine.
    *   This function loops indefinitely, reading `Job` structs from the channel.
    *   For now, the worker just prints the received job details (`fmt.Printf("Processing job: %+v\n", job)`).
    *   Start this worker goroutine when the application starts.
    *   *Commit Message:* `feat: Implement basic single worker processing jobs from channel`

*   **Phase 1 Complete**
    *   *Commit Message:* `feat: Complete Phase 1 - Basic API, worker, and in-memory queue`

---

### Phase 2: Persistence & Redis Queue

**Goal:** Replace the in-memory queue with Redis and persist job details and status in PostgreSQL.

*   [✅] **Step 2.1: Database Setup (PostgreSQL)**
    *   Create a `docker-compose.yml` file to run PostgreSQL.
    *   Define the SQL schema for a `jobs` table (e.g., `id UUID PRIMARY KEY`, `type VARCHAR(255)`, `payload JSONB`, `status VARCHAR(50)`, `created_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`). Create an SQL migration file.
    *   *Commit Message:* `chore: Add PostgreSQL container and define jobs table schema`
*   [✅] **Step 2.2: Database Integration**
    *   Add a database connection package (`internal/database`) using `database/sql` and a driver like `pgx`.
    *   Load DB connection details from configuration (env vars or config file).
    *   Modify `POST /jobs` handler:
        *   Generate a unique Job ID (e.g., UUID).
        *   Insert the new job into the `jobs` table with `status = 'pending'`.
    *   *Commit Message:* `feat: Integrate PostgreSQL, persist jobs on submission`
*   [✅] **Step 2.3: Redis Setup**
    *   Add Redis to the `docker-compose.yml`.
    *   Add a Redis client library (e.g., `go-redis/redis`).
    *   Connect to Redis, loading details from configuration.
    *   *Commit Message:* `chore: Add Redis container and client integration`
*   [✅] **Step 2.4: Redis Queue Implementation**
    *   Modify `POST /jobs` handler: After successfully inserting into PostgreSQL, push the `job_id` onto a Redis list (e.g., `taskq:pending_jobs`) using `LPUSH`.
    *   Modify the worker:
        *   Remove the Go channel logic.
        *   Use Redis `BRPOP` (blocking pop) to wait for and retrieve a `job_id` from the `taskq:pending_jobs` list.
    *   *Commit Message:* `feat: Implement Redis list-based queue using LPUSH/BRPOP`
*   [✅] **Step 2.5: Worker Database Interaction**
    *   When the worker receives a `job_id` from Redis:
        *   Query the PostgreSQL `jobs` table to fetch the full job details (type, payload) using the ID.
        *   Update the job status to `'running'` in the database.
        *   *Simulate work:* Add a `time.Sleep(2 * time.Second)`.
        *   After simulated work, update the job status to `'completed'` in the database.
    *   *Commit Message:* `feat: Worker fetches job details from DB and updates status`

*   **Phase 2 Challenge:**
    *   **Problem:** What happens if the worker process crashes *after* `BRPOP` removes the `job_id` from Redis, but *before* it successfully updates the status to `'completed'` or `'failed'` in PostgreSQL?
    *   **Task:** Research and think about how to make this more reliable. Look into Redis "Reliable Queue" patterns (e.g., using `RPOPLPUSH` to move the item to a temporary 'processing' list) or features of more advanced queues like NATS/RabbitMQ (acknowledgements). You don't have to implement it *yet*, but understand the problem.

*   **Phase 2 Complete**
    *   *Commit Message:* `feat: Complete Phase 2 - PostgreSQL persistence and Redis queue`

---

### Phase 3: Robustness, Error Handling & API Enhancements

**Goal:** Make the system more robust with multiple workers, proper error handling, basic retries, and an API to check job status.

*   [✅🚧] **Step 3.1: Worker Pool**
    *   Refactor the worker logic to allow running multiple worker goroutines concurrently.
    *   Use a `sync.WaitGroup` to manage the lifecycle of worker goroutines.
    *   Configure the number of workers via application settings.
    *   Ensure each worker independently loops on `BRPOP`.
    *   *Commit Message:* `feat: Implement concurrent worker pool`
*   [✅] **Step 3.2: Job Execution Logic & Error Handling**
    *   Create a simple job registry or map (`map[string]func(payload json.RawMessage) error`) to associate job `type` strings with actual Go functions.
    *   Implement 1-2 dummy job functions (e.g., `handleEmailJob`, `handleReportJob`). One should sometimes return an error.
    *   In the worker, look up the function based on `job.Type` and execute it.
    *   If the job function returns an error:
        *   Update the job status to `'failed'` in the database.
        *   Store the error message in a new `error_message TEXT` column in the `jobs` table.
    *   *Commit Message:* `feat: Implement job execution registry and handle job failures`
*   [ ] **Step 3.3: Basic Retry Mechanism**
    *   Add an `attempts INT DEFAULT 0` column to the `jobs` table.
    *   When a job fails:
        *   Increment the `attempts` count in the database.
        *   If `attempts` is less than a configured maximum (e.g., 3):
            *   Update status back to `'pending'`.
            *   Push the `job_id` back onto the `taskq:pending_jobs` Redis list (using `LPUSH`).
        *   If `attempts` reaches the maximum, leave status as `'failed'`.
    *   *Commit Message:* `feat: Implement basic retry mechanism for failed jobs`
*   **Phase 3 Challenge:**
    *   **Problem:** Pushing a failed job immediately back onto the main queue for retry might cause rapid failure loops if the underlying issue persists. Also, using `LPUSH` for retries puts it at the front, potentially blocking new jobs.
    *   **Task:** How could you implement a *delayed* retry? Think about using Redis Sorted Sets (score = retry timestamp) or simply having the worker sleep *before* re-queueing (less ideal as it blocks the worker). Consider how different queue technologies handle delayed messages.
*   [ ] **Step 3.4: Job Status API Endpoint**
    *   Implement a `GET /jobs/{job_id}` endpoint.
    *   This endpoint queries the PostgreSQL database for the job with the given ID.
    *   It returns the full job details, including its current `status`, `result` (if any - maybe add a `result JSONB` column later), or `error_message`.
    *   *Commit Message:* `feat: Implement GET /jobs/{job_id} endpoint for status check`

*   **Phase 3 Complete**
    *   *Commit Message:* `feat: Complete Phase 3 - Worker pool, error handling, retries, status API`

---

### Phase 4: Advanced Features & Polish (Future Extensions)

**Goal:** Add more sophisticated features, testing, and prepare for deployment.

*   [ ] **Step 4.1: Graceful Shutdown**
    *   Implement signal handling (SIGINT, SIGTERM) to allow the API server and workers to shut down gracefully.
    *   Workers should ideally finish their current job before exiting. Use `context.Context` for cancellation.
*   [ ] **Step 4.2: Unit & Integration Testing**
    *   Write unit tests for key logic (e.g., job functions, retry logic).
    *   Write integration tests for API endpoints (using `net/http/httptest`).
*   [ ] **Step 4.3: Configuration Management**
    *   Use a library like Viper to manage configuration cleanly (from file, env vars).
*   [ ] **Step 4.4: Real-time Updates (Optional)**
    *   Implement WebSockets or Server-Sent Events (SSE) to push job status updates to interested clients (e.g., a future frontend).
*   [ ] **Step 4.5: Scheduled Jobs (Optional)**
    *   Add functionality to submit jobs that should only run at a specific time in the future or on a recurring schedule (cron-like). This is significantly more complex.
*   [ ] **Step 4.6: Alternative Queue Backend (Optional)**
    *   Refactor the queue interaction into an interface.
    *   Implement the interface using NATS or RabbitMQ to explore their features (e.g., proper message acknowledgements, dead-letter queues, persistent topics).
*   [ ] **Step 4.7: Basic Frontend UI (Optional)**
    *   Build a simple React/Vue/Svelte or HTMX frontend to submit jobs and view their status by polling the API or using WebSockets/SSE if implemented.
*   [ ] **Step 4.8: Containerize for Production**
    *   Write a `Dockerfile` for the Go application.
    *   Ensure `docker-compose.yml` is suitable for development and potentially adaptable for production-like environments.

---

## Notes & Reminders

*   Remember to write clear, idiomatic Go code.
*   Add comments where necessary.
*   Keep the `README.md` updated with setup instructions and project overview.
*   Commit frequently with meaningful messages!