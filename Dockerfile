# ---- frontend ----
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
# vite writes to ../backend/web/dist
RUN mkdir -p /src/backend/web && npm run build

# ---- backend ----
FROM golang:1.22-alpine AS go
WORKDIR /src/backend
COPY backend/ ./
COPY --from=web /src/backend/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/riderl ./cmd/server

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/riderl /riderl
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/riderl"]
