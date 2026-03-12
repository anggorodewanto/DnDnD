FROM golang:1-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /dndnd ./cmd/dndnd/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /dndnd /dndnd

# Create the asset storage directory (mounted as a Fly Volume in production)
RUN mkdir -p /data/assets

EXPOSE 8080
CMD ["/dndnd"]
