# 第一階段:編譯
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/mes ./cmd/mes

# 第二階段:只留編譯好的程式,映像檔很小
FROM alpine:3.20
COPY --from=build /out/mes /mes
EXPOSE 8080
ENTRYPOINT ["/mes"]