FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY go.sum* ./
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api && CGO_ENABLED=0 go build -o /worker ./cmd/worker && CGO_ENABLED=0 go build -o /assert-queue ./cmd/assert-queue

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /api /worker /assert-queue /usr/local/bin/
WORKDIR /app
EXPOSE 8080
