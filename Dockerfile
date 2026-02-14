FROM alpine:latest

RUN apk add --no-cache ca-certificates libc6-compat

COPY dist/ /usr/local/bin/
COPY config.yaml /etc/hunter3/config.yaml

ENTRYPOINT ["hunter3"]
