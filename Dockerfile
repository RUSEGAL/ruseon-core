# Stage 1: Build Frontend
FROM node:24-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Backend
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ruseon-core ./cmd/server

# Stage 3: Final Image
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup
WORKDIR /app
COPY --from=backend-builder /app/ruseon-core /app/ruseon-core
RUN mkdir -p /app/data /app/recordings && \
    chown -R appuser:appgroup /app
USER appuser:appgroup
EXPOSE 8080
VOLUME ["/app/data", "/app/recordings"]
CMD ["/app/ruseon-core"]
