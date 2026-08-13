#!/bin/sh
# Test fixture callout for calloutClassifier.
#
# It is not anybody's real risk-classification policy — it exists to prove the
# DESK's half of the Q1 contract: that the request lands on stdin, that the
# response on stdout is read, and that both of the protocol's answers reach the
# classifier unmolested. It answers TRUE (block eligible / risk-classed) unless
# the environment says otherwise, so a test that forgets to set the toggle fails
# safe rather than silently exercising the wrong arm.
#
# Protocol (Q1, frozen in docs/streams/desk-risk-extension/brief-02-callout-classifier.md):
#   invoked as `<this> classify`; stdin carries {"repo":...,"changedFiles":[...]};
#   stdout must be exactly one line of {"riskClassed":bool,"reason":"..."}.
#
# Toggle: RISKCALLOUT_FIXTURE_ANSWER=classed|not-classed (default: classed).

cat >/dev/null # drain stdin — the request payload is not consulted by this fixture

case "$RISKCALLOUT_FIXTURE_ANSWER" in
  not-classed)
    printf '{"riskClassed":false,"reason":"fixture: this diff cleared the adopter policy"}\n'
    ;;
  *)
    printf '{"riskClassed":true,"reason":"fixture: adopter policy flagged this diff"}\n'
    ;;
esac
