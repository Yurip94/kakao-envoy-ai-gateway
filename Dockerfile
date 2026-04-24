FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /memory-extproc ./cmd/memory-extproc

FROM alpine:3.20
COPY --from=builder /memory-extproc /memory-extproc
EXPOSE 50051
ENTRYPOINT ["/memory-extproc"]
