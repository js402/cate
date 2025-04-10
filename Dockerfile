FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc g++ musl-dev #https://github.com/ollama/ollama/pull/8106

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o cate

FROM alpine:3.19
RUN apk add --no-cache libstdc++ libgcc # ? #https://github.com/ollama/ollama/pull/8106
WORKDIR /

COPY --from=builder / /app/

ENTRYPOINT ["/cate"]
