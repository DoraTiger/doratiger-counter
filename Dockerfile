FROM alpine:3.20

ARG TARGETARCH
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S counter \
    && adduser -S -G counter -h /app counter \
    && install -d -o counter -g counter /app/data
WORKDIR /app
COPY build/linux-${TARGETARCH}/doratiger-counter /usr/local/bin/doratiger-counter
COPY --chown=counter:counter config.toml /app/config.toml
USER counter
VOLUME ["/app/data"]
EXPOSE 8080
ENTRYPOINT ["doratiger-counter"]
CMD ["serve", "--config", "/app/config.toml"]
