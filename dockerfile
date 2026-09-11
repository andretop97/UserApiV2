FROM golang:1.26.2 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /UserApiv2 ./

FROM alpine:3.21

RUN apk --no-cache add curl

COPY --from=builder /UserApiv2 /UserApiv2

COPY migrations /migrations

EXPOSE 8080

CMD [ "/UserApiv2" ]