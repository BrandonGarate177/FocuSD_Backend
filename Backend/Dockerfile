# Build stage
FROM golang:1.24 as builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o server main.go

# Run stage
FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /app/server ./
COPY --from=builder /app/uploads ./uploads
COPY --from=builder /app/.env ./
ENV PORT=8080
EXPOSE 8080
CMD ["/app/server"]

