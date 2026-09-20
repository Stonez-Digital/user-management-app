FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN apk add --no-cache gcc musl-dev
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /out/school-api ./cmd/server

FROM alpine:3.22
RUN addgroup -S app && adduser -S -G app app && apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/school-api /app/school-api
USER app
EXPOSE 8080
ENV APP_PORT=8080
ENTRYPOINT ["/app/school-api"]
