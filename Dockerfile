# Build Stage
FROM golang:1.22.5-alpine3.20 AS Build
WORKDIR /app
COPY . .
RUN go build -o main main.go

# Release Stage
FROM alpine:3.20.2 AS Release
WORKDIR /app
COPY --from=Build /app/main .

EXPOSE 8080
CMD [ "/app/main" ]