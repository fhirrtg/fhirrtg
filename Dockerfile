FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY lib/go.mod ./
RUN go mod download

# Copy source code
COPY lib/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main .

# Final stage
FROM alpine:3

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/main .

# Run the application
ENTRYPOINT ["./main"]