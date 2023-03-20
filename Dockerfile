# Start from the official golang base image
FROM golang:1.17-alpine as builder

# Add the required build tools
RUN apk add --no-cache git

# Set the working directory
WORKDIR /app

# Copy the Go source files
COPY . .
COPY go.mod .

# Build the Go program
RUN CGO_ENABLED=0 go build -o rpc-proxy

# Use the alpine base image for the final, smaller image
FROM alpine:3.14

# Set the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/rpc-proxy /app/rpc-proxy

# Expose the default listen port
EXPOSE 10545

# Run the Go program
CMD ["/app/rpc-proxy"]
