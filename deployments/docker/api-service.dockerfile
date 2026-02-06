# # create temporary builder container to build the target image in a consistent, reproducible environment
# FROM golang:1.24 AS builder
# WORKDIR /app

# # copy go mod files first for better caching
# COPY go.mod go.sum ./
# RUN go mod download && go mod verify

# # add requirements for mattn/go-sqlite
# # RUN apk add gcc musl-dev
# ENV CGO_ENABLED=1

# # copy project source code
# COPY . .

# # build the binary
# RUN cd cmd/api && go build -tags "linux sqlite_fts5" -o /app/apiService

# # build a docker image, which does not include go or anything - just the smallest possible image to run the binary on. this image is where the service is hosted
# FROM gcr.io/distroless/base
# # FROM alpine:latest

# ARG PROJECT_NAME

# # already included in image
# # install runtime dependencies - allows app to make https requests and deal with timezones outside of utc
# # RUN apk --no-cache add ca-certificates tzdata

# # set working directory
# WORKDIR /app

# # copy binary from builder
# COPY --from=builder /app/apiService .

# # USER root
# # RUN mkdir -p /app/db && chown nonroot:nonroot /app/db
# USER nonroot

# # run the app
# CMD [ "./apiService" ]

FROM golang:1.24-alpine AS builder

WORKDIR /app
RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=1
# RUN go build -o apiService ./cmd/api
RUN go build -tags "linux sqlite_fts5" -o apiService ./cmd/api


FROM alpine

ARG PROJECT_NAME

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/apiService .

CMD ["./apiService"]