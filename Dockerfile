ARG GO_VERSION=1.14.1

FROM golang:${GO_VERSION}-buster as builder

WORKDIR /src

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
    -installsuffix 'static' \
    -o /app .

#COPY --from=builder /user/group /user/passwd /etc/
#COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
#COPY --from=builder /app /app

EXPOSE 8080

ENTRYPOINT ["/app"]

