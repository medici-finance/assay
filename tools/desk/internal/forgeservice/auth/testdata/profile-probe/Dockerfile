FROM scratch
COPY --chown=501:501 probe /probe-bin
COPY --chown=501:501 private/ /probe/private/
COPY --chown=501:501 channel/ /probe/channel/
USER 501:501
ENTRYPOINT ["/probe-bin"]
