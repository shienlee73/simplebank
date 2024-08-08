# Build Stage
FROM golang:1.22.5-alpine3.20 AS build
WORKDIR /app
COPY . .
RUN go build -o main main.go

# Release Stage
FROM alpine:3.20.2 AS release
WORKDIR /app
COPY --from=build /app/main .
COPY --from=build /app/app.env .
COPY --from=build /app/start.sh .
COPY --from=build /app/db/migration ./db/migration

RUN chmod +x start.sh

EXPOSE 8080 9090
ENTRYPOINT [ "/app/start.sh" ]
CMD [ "/app/main" ]