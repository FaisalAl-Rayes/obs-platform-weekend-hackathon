FROM golang:1.25 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /ria ./cmd/ria

FROM gcr.io/distroless/static
COPY --from=build /ria /ria
EXPOSE 8080
VOLUME /data
CMD ["/ria"]
