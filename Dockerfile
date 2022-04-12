## Build stage
FROM golang:1.16-alpine AS build
WORKDIR /app
ENV CGO_ENABLED=0

# Download deps
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Build app
COPY auth auth
COPY cmd cmd
COPY config config
COPY db db
COPY doc doc
COPY helper helper
COPY receiver receiver
COPY rest rest
COPY track track
#COPY *.go ./
RUN go build -v -o techo-backend cmd/main/main.go

# Test
# TODO add tests
#RUN go test -v ./...

## Runtime stage
FROM alpine:3 AS runtime
WORKDIR /app

COPY --from=build /app/techo-backend ./

ENTRYPOINT ["./techo-backend"]
CMD [""]
