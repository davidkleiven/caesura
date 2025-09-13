FROM scratch

FROM scratch

LABEL org.opencontainers.image.title="Caesura"
LABEL org.opencontainers.image.description="Caesura is the modern solution for orchestras, bands, and choirs to store, manage, and distribute music"
LABEL org.opencontainers.image.licenses="BSL-1.1"
LABEL org.opencontainers.image.source="https://github.com/davidkleiven/caesura"

COPY caesura /caesura
COPY LICENSE /LICENSE

ENTRYPOINT ["/caesura"]
COPY caesura /caesura
COPY LICENSE /LICENSE
ENTRYPOINT ["/caesura"]
