
# docker build . -t strangelove/full-celestia:v0.0.1
#
# Run both a celestia-da and celestia-app node from the same image
# Ideally this is temperary so we can ignore docker host mounting.

FROM ghcr.io/strangelove-ventures/heighliner/celestia:v1.6.0 AS celestia-app

FROM ghcr.io/rollkit/celestia-da:v0.12.6

USER root

COPY --from=celestia-app /bin/celestia-appd /bin/

RUN chmod +x /bin/celestia-appd
RUN chmod +x /bin/celestia-da

EXPOSE 26650 26657 26658 26659 9090 26657 26656 1317


# docker run reece/full-celestia:v0.0.1 celestia-da