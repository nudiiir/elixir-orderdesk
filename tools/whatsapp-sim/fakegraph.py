"""A stand-in for Meta's Graph API, for testing the WhatsApp channel without a WhatsApp account:
it records every message the desk sends (outbox.jsonl) and serves media files from ./media.

    python tools/whatsapp-sim/fakegraph.py 8699

Order Desk is then started with WHATSAPP_GRAPH_URL=http://127.0.0.1:8699, WHATSAPP_TOKEN=TOKEN,
WHATSAPP_PHONE_ID=PHONE, WHATSAPP_VERIFY_TOKEN=VERIFY and WHATSAPP_APP_SECRET=SECRET (see docs/WHATSAPP.md)."""
import json, os, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

PORT = int(sys.argv[1]) if len(sys.argv) > 1 else 8699
HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "outbox.jsonl")


class H(BaseHTTPRequestHandler):
    def _json(self, code, obj):
        b = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        if self.headers.get("Authorization") != "Bearer TOKEN":
            return self._json(401, {"error": {"message": "bad token", "code": 190}})
        if self.path.endswith("/messages"):
            m = json.loads(body)
            if m.get("status") != "read":
                with open(OUT, "a", encoding="utf-8") as f:
                    f.write(json.dumps(m, ensure_ascii=False) + "\n")
            return self._json(200, {"messages": [{"id": "wamid.out"}]})
        self._json(404, {"error": {"message": "unknown"}})

    def do_GET(self):
        if self.path.startswith("/files/"):
            p = os.path.join(HERE, "media", os.path.basename(self.path))
            if not os.path.exists(p):
                return self._json(404, {})
            b = open(p, "rb").read()
            self.send_response(200)
            self.send_header("Content-Length", str(len(b)))
            self.end_headers()
            return self.wfile.write(b)
        if self.headers.get("Authorization") != "Bearer TOKEN":
            return self._json(401, {"error": {"message": "bad token", "code": 190}})
        if self.path.startswith("/PHONE"):
            return self._json(200, {"display_phone_number": "+213 555 00 00 00", "verified_name": "Order Desk (test)"})
        media = self.path.strip("/").split("?")[0]
        return self._json(200, {"url": f"http://127.0.0.1:{PORT}/files/{media}", "mime_type": "image/png"})

    def log_message(self, *a):
        pass


ThreadingHTTPServer(("127.0.0.1", PORT), H).serve_forever()
