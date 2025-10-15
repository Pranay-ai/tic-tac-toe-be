# Stage 1: Build the Go binary
# We use a specific Go version on a minimal OS (alpine) to compile the app.
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy dependency files first to leverage Docker's layer caching
COPY go.mod go.sum ./
# Download the Go module dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Compile the Go application into a single, static binary named "server"
# CGO_ENABLED=0 is important for creating a truly static binary
# GOOS=linux ensures it's built for a Linux container environment
RUN CGO_ENABLED=0 GOOS=linux go build -o /server .


# Stage 2: Create the final, lightweight image
# We start from a fresh, minimal base image for security and small size.
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy ONLY the compiled binary from the previous "builder" stage
COPY --from=builder /server .

# Tell Docker that the container listens on port 8080 at runtime
EXPOSE 8080

# The command to run when the container starts
CMD ["./server"]