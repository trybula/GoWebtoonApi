# Use Go 1.23 bookworm as base image
FROM golang:1.25.5-alpine AS base

# Move to working directory /build
WORKDIR /build

RUN mkdir lib comics
RUN chmod -R 777 comics
# Copy the go.mod and go.sum files to the /build directory
COPY go.mod go.sum ./

# Install dependencies
RUN go mod download

COPY . .

# Build the application
RUN go build -o go-webtoon

# Document the port that may need to be published
EXPOSE 80

# Start the application
CMD ["/build/go-webtoon"]
