FROM golang:1.23-alpine3.20 AS build

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 go build -o /app/bin/yukari .

FROM alpine:3.20

COPY --from=build /app/bin/yukari /app/bin/yukari

ENV BIND=:9200
ENV MANIFEST_LIFETIME=240h
ENV SLOG_LEVEL=ERROR
ENV UPSTREAM_REGISTRY=https://registry.ollama.ai/

EXPOSE 9200

ENTRYPOINT ["/app/bin/yukari"]
