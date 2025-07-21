# Use an official Golang image as a parent image
FROM golang:1.20-alpine

# Set the working directory in the container to /app
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go application
RUN go mod verify && go build -o main ./cmd/root.go

# Expose port 8080 for the app
EXPOSE 8080

# Command to run the executable
CMD ["./main"]
