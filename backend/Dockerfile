FROM golang:1.26-alpine AS build
ENV GOPROXY https://goproxy.cn,direct
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache tzdata && adduser -D -H appuser
ENV TZ=Asia/Shanghai
USER appuser
COPY --from=build /server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
