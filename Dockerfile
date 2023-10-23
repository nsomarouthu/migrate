FROM golang:1.20-alpine3.18 AS builder
ARG VERSION
ARG DATABASE
ARG SOURCE

RUN apk add --no-cache git gcc musl-dev make

WORKDIR /go/src/github.com/infobloxopen/migrate

ENV GO111MODULE=on

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 go build -a -o build/migrate.linux-386 -ldflags="-s -w -X main.Version=${VERSION}" -tags "$(DATABASE) $(SOURCE)" ./cmd/migrate

FROM alpine:latest

COPY --from=builder /go/src/github.com/infobloxopen/migrate/cmd/migrate/config /cli/config/
COPY --from=builder /go/src/github.com/infobloxopen/migrate/build/migrate.linux-386 /usr/local/bin/migrate
RUN ln -s /usr/local/bin/migrate /migrate

ENTRYPOINT ["migrate"]
CMD ["--help"]
