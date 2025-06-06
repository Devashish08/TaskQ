# Dockerfile

# ---- Build Stage ----
    FROM golang:1.24-alpine AS builder

    WORKDIR /app
    
    COPY go.mod go.sum ./
    RUN go mod download
    RUN go mod verify
    
    COPY . .
    
    RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/taskq-server ./cmd/taskq-server
    
    FROM alpine:latest

    WORKDIR /app
    
    COPY --from=builder /app/taskq-server /app/taskq-server
   
    
    EXPOSE 8080
    
    ENTRYPOINT ["/app/taskq-server"]