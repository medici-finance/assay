FROM scratch
COPY --chown=501:501 --chmod=0700 seeds/ /
COPY --chown=501:501 authority-helper /authority-helper
USER 501:501
ENTRYPOINT ["/authority-helper"]
