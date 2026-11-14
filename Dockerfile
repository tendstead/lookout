FROM node:26.13.1-alpine AS frontend
WORKDIR /src/web
RUN npm install --global npm@12.2.2 --no-audit --no-fund
COPY web/package*.json ./
COPY web/scripts/ ./scripts/
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build && node scripts/collect-notices.mjs

FROM golang:1.27.4 AS backend
WORKDIR /src
COPY go.mod go.sum ./
COPY scripts/collect-go-notices.sh /tmp/collect-go-notices.sh
RUN go mod download && sh /tmp/collect-go-notices.sh
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=frontend /src/cmd/server/static/ ./cmd/server/static/
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /lookout ./cmd/server

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=backend /lookout /app/lookout
COPY LICENSE /app/LICENSE
COPY docs/third-party/ /app/third-party/
COPY --from=frontend /src/.implementation/notices/ /app/third-party/
COPY --from=backend /notices/ /app/third-party/
ENV LISTEN_ADDR=:8080 GOMEMLIMIT=96MiB GOMAXPROCS=1
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/lookout"]
