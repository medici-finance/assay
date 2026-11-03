"""Client-side SCRAM-SHA-256 verifier for PostgreSQL (docs/postgres.md).

Reads ONE password line on stdin and prints the verifier PostgreSQL stores in pg_authid:

    SCRAM-SHA-256$<iterations>:<salt>$<StoredKey>:<ServerKey>

which is what psql's \\password sends. provision-cell.sh and restore.sh send only this string to
the server as a role's PASSWORD, so the plaintext never reaches the server, its log or
pg_stat_statements. The password is never taken from argv or the environment.

`selftest` checks the implementation against the RFC 7677 test vector first; both scripts run
it and refuse, rather than fall back to sending plaintext, when it fails.

Run with `python3 -I` (isolated mode: no user site-packages, no script-directory imports).
Standard library only.
"""

import base64
import hashlib
import hmac
import os
import re
import sys

ITERATIONS = 4096  # PostgreSQL's default scram_iterations
SALT_BYTES = 16  # what PostgreSQL itself generates


def _keys(password: bytes, salt: bytes, iterations: int):
    # The passwords this house generates are [A-Za-z0-9_-]: SASLprep leaves them unchanged.
    salted = hashlib.pbkdf2_hmac("sha256", password, salt, iterations)
    client_key = hmac.new(salted, b"Client Key", hashlib.sha256).digest()
    stored_key = hashlib.sha256(client_key).digest()
    server_key = hmac.new(salted, b"Server Key", hashlib.sha256).digest()
    return client_key, stored_key, server_key


def verifier(password: bytes, salt: bytes, iterations: int = ITERATIONS) -> str:
    _, stored_key, server_key = _keys(password, salt, iterations)
    b64 = lambda b: base64.b64encode(b).decode("ascii")
    return "SCRAM-SHA-256$%d:%s$%s:%s" % (iterations, b64(salt), b64(stored_key), b64(server_key))


def selftest() -> bool:
    # RFC 7677 §3: user "user", password "pencil".
    salt = base64.b64decode("W22ZaJ0SNY7soEsUEjb6gQ==")
    client_first_bare = b"n=user,r=rOprNGfwEbeRWgbNEkqO"
    server_first = (
        b"r=rOprNGfwEbeRWgbNEkqO%hvYDpWUa2RaTCAfuxFIlj)hNlF$k0,"
        b"s=W22ZaJ0SNY7soEsUEjb6gQ==,i=4096"
    )
    client_final_bare = b"c=biws,r=rOprNGfwEbeRWgbNEkqO%hvYDpWUa2RaTCAfuxFIlj)hNlF$k0"
    auth_message = client_first_bare + b"," + server_first + b"," + client_final_bare
    client_key, stored_key, server_key = _keys(b"pencil", salt, 4096)
    client_signature = hmac.new(stored_key, auth_message, hashlib.sha256).digest()
    proof = bytes(a ^ b for a, b in zip(client_key, client_signature))
    server_signature = hmac.new(server_key, auth_message, hashlib.sha256).digest()
    # The RFC's expected values, as hex of the base64 it prints:
    #   proof            p=dHzbZapW...AndVQ=
    #   server signature v=6rriTRBi...l95G4=
    return (
        proof.hex() == "747cdb65aa56224e2352137e52d7bdcad6a0f738df30782caa69a2cfb0277554"
        and server_signature.hex() == "eabae24d1062db75a9451ff0b6ea7e98c8546549ff741e672d3251b2397de46e"
    )


def main(argv) -> int:
    if argv[1:] == ["selftest"]:
        if not selftest():
            sys.stderr.write("scram-verifier: RFC 7677 self-test FAILED\n")
            return 1
        return 0
    if argv[1:]:
        sys.stderr.write("usage: python3 -I scram-verifier.py [selftest] < password-line\n")
        return 2
    line = sys.stdin.buffer.readline().rstrip(b"\r\n")
    if not re.fullmatch(rb"[A-Za-z0-9_-]{32,}", line):
        sys.stderr.write("scram-verifier: refusing a password outside [A-Za-z0-9_-]{32,}\n")
        return 1
    sys.stdout.write(verifier(line, os.urandom(SALT_BYTES)) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
