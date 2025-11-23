FROM scratch

LABEL org.opencontainers.image.title="Caesura"
LABEL org.opencontainers.image.description="Caesura is the modern solution for orchestras, bands, and choirs to store, manage, and distribute music"
LABEL org.opencontainers.image.licenses="BSL-1.1"
LABEL org.opencontainers.image.source="https://github.com/davidkleiven/caesura"

COPY caesura /caesura
COPY LICENSE /LICENSE

COPY --from=golang:1.24-bullseye /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["/caesura"]
