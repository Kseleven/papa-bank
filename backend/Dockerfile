# 构建上下文为项目根目录(见 docker-compose.yml)
# ---- 构建阶段 ----
FROM golang:1.22.12-alpine3.21 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/*.go ./
RUN CGO_ENABLED=0 go build -o /app/server .

# ---- 运行阶段 ----
FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache tzdata ca-certificates
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=build /app/server .
COPY web ./web
EXPOSE 8080
CMD ["./server"]
