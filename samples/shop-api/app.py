#!/usr/bin/env python3
"""Tiny shop backend. Reads credentials from the process environment only.

It does not load .env files. That is hush's job:

    hush run -- python3 app.py
    hush run -- python3 app.py serve
"""

from __future__ import annotations

import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

REQUIRED = ("DATABASE_URL", "STRIPE_SECRET_KEY", "SENDGRID_API_KEY")


def mask(value: str, keep: int = 4) -> str:
    if len(value) <= keep:
        return "•" * len(value)
    return value[:keep] + "•" * max(4, len(value) - keep)


def load_config() -> dict[str, str]:
    missing = [name for name in REQUIRED if not os.environ.get(name)]
    if missing:
        names = ", ".join(missing)
        sys.stderr.write(
            f"missing required secrets: {names}\n"
            "this process does not read .env — run it with:\n"
            "  hush run -- python3 app.py\n"
        )
        sys.exit(1)
    return {name: os.environ[name] for name in REQUIRED}


def describe(cfg: dict[str, str]) -> str:
    db = urlparse(cfg["DATABASE_URL"])
    host = db.hostname or "?"
    dbname = (db.path or "/").lstrip("/") or "?"
    lines = [
        "shop-api  ready",
        f"  database  {db.scheme}://{host}/{dbname}",
        f"  stripe    {mask(cfg['STRIPE_SECRET_KEY'])}",
        f"  sendgrid  {mask(cfg['SENDGRID_API_KEY'])}",
    ]
    return "\n".join(lines) + "\n"


class Handler(BaseHTTPRequestHandler):
    cfg: dict[str, str] = {}

    def log_message(self, fmt: str, *args) -> None:
        sys.stderr.write("%s - %s\n" % (self.address_string(), fmt % args))

    def do_GET(self) -> None:
        if self.path != "/health":
            self.send_error(404)
            return
        body = describe(self.cfg).encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def serve(cfg: dict[str, str]) -> None:
    Handler.cfg = cfg
    port = int(os.environ.get("PORT", "8787"))
    httpd = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    sys.stdout.write(describe(cfg))
    sys.stdout.write(f"listening  http://127.0.0.1:{port}/health\n")
    sys.stdout.flush()
    httpd.serve_forever()


def main() -> None:
    cfg = load_config()
    if len(sys.argv) > 1 and sys.argv[1] == "serve":
        serve(cfg)
        return
    sys.stdout.write(describe(cfg))


if __name__ == "__main__":
    main()
