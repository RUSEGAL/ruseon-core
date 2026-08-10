# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder
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
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o reastream ./cmd/server

# Stage 3: Final Image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /app/reastream /app/reastream
COPY --from=frontend-builder /app/web/dist /app/web/dist
RUN mkdir -p /app/data /app/recordings /app/backups
EXPOSE 8080
VOLUME ["/app/data", "/app/recordings", "/app/backups"]
CMD ["/app/reastream"]
