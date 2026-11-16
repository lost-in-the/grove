"""A stateful fake of the slice of the GitHub API update-homebrew-tap.sh uses.

Used by scripts/test-update-homebrew-tap.sh; the release job it tests only runs
on a tag push, so this is the only place its logic runs before a release.

POST /__reset with a JSON config resets state; GET /__state dumps it.
Config keys: main_formula (text), put_status (int), contents_status (int, for
GET contents on the default branch).
A request the script should never make (a merge, say) gets a 404 and still
shows up in the recorded calls.

The tap's rule is modelled: a direct write to main is refused with 409.
"""
import base64, hashlib, json, sys, threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse, parse_qs

TAP = "lost-in-the/homebrew-tap"
PATH = "Formula/grove.rb"
state = {}
lock = threading.Lock()


def reset(cfg):
    state.clear()
    state.update(cfg=cfg, refs={"main": "base0"}, files={"main": cfg.get("main_formula")},
                 pulls=[], calls=[])


def blob_sha(text):
    return hashlib.sha1(text.encode()).hexdigest()


class H(BaseHTTPRequestHandler):
    def log_message(self, *a):
        pass

    def reply(self, code, obj):
        body = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def body(self):
        n = int(self.headers.get("Content-Length") or 0)
        return json.loads(self.rfile.read(n) or b"{}")

    def handle_any(self, method):
        u = urlparse(self.path)
        q = parse_qs(u.query)
        with lock:
            if u.path == "/__reset":
                reset(self.body()); return self.reply(200, {})
            if u.path == "/__state":
                return self.reply(200, state)
            state["calls"].append(f"{method} {u.path}")
            cfg = state["cfg"]
            if method == "GET" and u.path == f"/repos/{TAP}":
                return self.reply(200, {"default_branch": "main"})
            if u.path == f"/repos/{TAP}/contents/{PATH}":
                if method == "GET":
                    ref = q["ref"][0]
                    if ref == "main" and cfg.get("contents_status"):
                        return self.reply(cfg["contents_status"], {"message": "boom"})
                    text = state["files"].get(ref)
                    if text is None:
                        return self.reply(404, {"message": "Not Found"})
                    return self.reply(200, {"sha": blob_sha(text),
                                            "content": base64.encodebytes(text.encode()).decode()})
                if method == "PUT":
                    b = self.body()
                    if cfg.get("put_status"):
                        return self.reply(cfg["put_status"], {"message": "forbidden"})
                    branch = b.get("branch", "main")
                    if branch == "main":
                        return self.reply(409, {"message": "Repository rule violations found"})
                    cur = state["files"].get(branch)
                    if cur is not None and b.get("sha") != blob_sha(cur):
                        return self.reply(409, {"message": "sha mismatch"})
                    state["files"][branch] = base64.b64decode(b["content"]).decode()
                    return self.reply(200 if cur is not None else 201, {"content": {}})
            if method == "GET" and u.path == f"/repos/{TAP}/git/ref/heads/main":
                return self.reply(200, {"object": {"sha": state["refs"]["main"]}})
            if method == "POST" and u.path == f"/repos/{TAP}/git/refs":
                b = self.body()
                name = b["ref"].removeprefix("refs/heads/")
                if name in state["refs"]:
                    return self.reply(422, {"message": "Reference already exists"})
                state["refs"][name] = b["sha"]
                state["files"][name] = state["files"]["main"]
                return self.reply(201, {"ref": b["ref"]})
            if u.path == f"/repos/{TAP}/pulls":
                if method == "GET":
                    head = q.get("head", [""])[0].split(":", 1)[-1]
                    return self.reply(200, [p for p in state["pulls"] if p["head"] == head and p["state"] == "open"])
                if method == "POST":
                    b = self.body()
                    n = len(state["pulls"]) + 6
                    pr = {"number": n, "head": b["head"], "state": "open",
                          "html_url": f"https://github.com/{TAP}/pull/{n}", "title": b["title"]}
                    state["pulls"].append(pr)
                    return self.reply(201, pr)
            return self.reply(404, {"message": f"fake has no {method} {u.path}"})

    def do_GET(self): self.handle_any("GET")
    def do_POST(self): self.handle_any("POST")
    def do_PUT(self): self.handle_any("PUT")


reset({})
ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1])), H).serve_forever()
