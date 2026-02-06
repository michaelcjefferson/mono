FROM golang:1.24-alpine AS builder

WORKDIR /app
# add dependencies required in order to build cgo apps
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=1
RUN go build -tags "linux sqlite_fts5" -o apiService ./cmd/api


FROM alpine

ARG PROJECT_NAME

WORKDIR /app
# add dependencies to work with timezones and make https requests
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/apiService .

CMD ["./apiService"]