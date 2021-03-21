## Build stage
FROM golang:1.16-alpine AS build
WORKDIR /app
ENV CGO_ENABLED=0

# Download deps
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Build app
COPY cmd cmd
COPY db db
COPY helper helper
COPY objects objects
COPY receiver receiver
COPY types types
COPY *.go ./
RUN go build -v -o gondulapi cmd/test/main.go

# Test
RUN go test -v .

## Runtime stage
FROM alpine:3 AS runtime
WORKDIR /app

COPY --from=build /app/gondulapi ./

ENTRYPOINT ["./gondulapi"]
CMD [""]
