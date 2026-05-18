# Step 1: Use the official Go image to compile our code
FROM golang:1.25-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files first to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build a super-optimized executable file named "engine"
RUN go build -o engine main.go

# Step 2: Create a tiny production image
FROM alpine:latest

WORKDIR /app

# Copy the compiled API binary (UI is served separately by the Next.js frontend)
COPY --from=builder /app/engine .

# Expose the API port
EXPOSE 8080

# Command to run the executable
CMD ["./engine"]