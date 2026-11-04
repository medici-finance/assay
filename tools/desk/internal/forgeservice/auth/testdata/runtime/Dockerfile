FROM scratch
COPY --chown=501:501 seeds/ /
COPY --chown=501:501 authority-helper /authority-helper
USER 501:501
ENTRYPOINT ["/authority-helper"]
