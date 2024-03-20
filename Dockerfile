FROM golang:1.20 as builder
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct
WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY *.go ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /requestBox
FROM gcr.io/distroless/static-debian11
ENV TZ=Asia/Shanghai
COPY --from=builder /requestBox /requestBox
EXPOSE 8080
CMD ["/requestBox"]