#!/usr/bin/env python3
"""Verify a signed release; publish only with explicit --publish.

The repository's embedded public key is the sole trust root. The release
directory and manifest cannot choose either a signing key or a destination.
"""
import argparse
import base64
import binascii
import hashlib
import http.client
import json
from pathlib import Path
import re
import secrets
import subprocess
import sys
import tempfile
import time
from urllib.parse import quote, urljoin, urlsplit


BASE_URL = "https://file.booyah.dev/BoopsDB-Client/"
TRUSTED_PUBLIC_KEY = Path(__file__).resolve().parents[1] / "update" / "public-key.pem"
MANIFEST_LIMIT = 1024 * 1024
BINARY_LIMIT = 64 * 1024 * 1024
BOOTSTRAP_NAMES = ("install_0.3.sh", "install.sh", "public-key.pem", "verification-ja.md", "SHA256SUMS")
VERSION = re.compile(r"(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\Z")


class PublishError(Exception):
    pass


def _unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise PublishError("duplicate JSON key: " + key)
        result[key] = value
    return result


def _json(data):
    def invalid_number(value):
        raise PublishError("invalid JSON number: " + value)

    try:
        return json.loads(data.decode("utf-8"), object_pairs_hook=_unique_object,
                          parse_constant=invalid_number)
    except (ValueError, UnicodeError, RecursionError) as error:
        raise PublishError("invalid UTF-8 JSON") from error


def _exact_keys(value, keys, description):
    if not isinstance(value, dict) or set(value) != set(keys):
        raise PublishError("invalid " + description + " fields")


def _read_local(path, limit):
    try:
        if path.is_symlink() or not path.is_file():
            raise PublishError("not a regular release file: " + path.name)
        if path.stat().st_size > limit:
            raise PublishError("release file exceeds size limit: " + path.name)
        with path.open("rb") as file:
            data = file.read(limit + 1)
        if len(data) > limit:
            raise PublishError("release file exceeds size limit: " + path.name)
        return data
    except OSError as error:
        raise PublishError("cannot read release file: " + path.name) from error


def verify_manifest(data):
    if len(data) > MANIFEST_LIMIT:
        raise PublishError("manifest exceeds 1 MiB")
    envelope = _json(data)
    _exact_keys(envelope, ("payload", "signature"), "manifest envelope")
    try:
        if not all(isinstance(envelope[key], str) for key in ("payload", "signature")):
            raise PublishError("manifest payload and signature must be Base64 strings")
        payload = base64.b64decode(envelope["payload"], validate=True)
        signature = base64.b64decode(envelope["signature"], validate=True)
    except (ValueError, binascii.Error) as error:
        raise PublishError("invalid manifest Base64") from error
    if (base64.b64encode(payload).decode("ascii") != envelope["payload"] or
            base64.b64encode(signature).decode("ascii") != envelope["signature"]):
        raise PublishError("manifest Base64 must use canonical standard encoding")
    if len(signature) != 64:
        raise PublishError("invalid Ed25519 signature length")
    try:
        with tempfile.TemporaryDirectory(prefix="boops-release-verify-") as temporary:
            directory = Path(temporary)
            (directory / "payload.json").write_bytes(payload)
            (directory / "signature.bin").write_bytes(signature)
            checked = subprocess.run([
                "openssl", "pkeyutl", "-verify", "-rawin", "-pubin", "-inkey", str(TRUSTED_PUBLIC_KEY),
                "-in", str(directory / "payload.json"), "-sigfile", str(directory / "signature.bin")],
                capture_output=True, timeout=15, check=False)
        if checked.returncode != 0:
            raise PublishError("signature verification with embedded public key failed")
    except (OSError, subprocess.TimeoutExpired) as error:
        raise PublishError("OpenSSL signature verification unavailable or timed out") from error
    manifest = _json(payload)
    _exact_keys(manifest, ("schema", "version", "artifacts"), "signed manifest")
    if type(manifest["schema"]) is not int or manifest["schema"] != 1:
        raise PublishError("unsupported manifest schema")
    version = manifest["version"]
    if not isinstance(version, str) or not VERSION.fullmatch(version):
        raise PublishError("version must have three canonical numeric components")
    if any(len(part) > 20 or int(part) > 18446744073709551615 for part in version.split(".")):
        raise PublishError("version component exceeds uint64 range")
    _exact_keys(manifest["artifacts"], ("linux/amd64", "linux/arm64"), "artifact platform")
    for arch in ("amd64", "arm64"):
        artifact = manifest["artifacts"]["linux/" + arch]
        _exact_keys(artifact, ("path", "size", "sha256"), "artifact")
        if artifact["path"] != "boops_" + version + "_" + arch + ".binary":
            raise PublishError("artifact path does not match version and architecture")
        if type(artifact["size"]) is not int or not 0 < artifact["size"] <= BINARY_LIMIT:
            raise PublishError("invalid artifact size")
        if not isinstance(artifact["sha256"], str) or not re.fullmatch(r"[0-9a-f]{64}", artifact["sha256"]):
            raise PublishError("invalid artifact SHA-256")
    return manifest


def _matches(data, artifact):
    return len(data) == artifact["size"] and hashlib.sha256(data).hexdigest() == artifact["sha256"]


def validate_release(directory):
    """Take a verified in-memory snapshot so later local changes cannot be uploaded."""
    directory = Path(directory)
    data = _read_local(directory / "latest.json", MANIFEST_LIMIT)
    manifest = verify_manifest(data)
    files = {}
    for artifact in manifest["artifacts"].values():
        contents = _read_local(directory / artifact["path"], BINARY_LIMIT)
        if not _matches(contents, artifact):
            raise PublishError("local binary size/SHA-256 mismatch: " + artifact["path"])
        files[artifact["path"]] = contents
    for name in BOOTSTRAP_NAMES:
        path = directory / name
        if path.exists() or path.is_symlink():
            files[name] = _read_local(path, MANIFEST_LIMIT)
    aliases = [name in files for name in ("install.sh", "install_0.3.sh")]
    if any(aliases) and (not all(aliases) or files["install.sh"] != files["install_0.3.sh"]):
        raise PublishError("both installer aliases must exist with identical bytes")
    if "public-key.pem" in files and files["public-key.pem"] != _read_local(TRUSTED_PUBLIC_KEY, MANIFEST_LIMIT):
        raise PublishError("release public key differs from embedded public key")
    if "SHA256SUMS" in files:
        try:
            lines = files["SHA256SUMS"].decode("ascii").splitlines()
        except UnicodeError as error:
            raise PublishError("checksum evidence must be ASCII") from error
        expected = {artifact["sha256"] + "  " + artifact["path"] for artifact in manifest["artifacts"].values()}
        if len(lines) != 2 or set(lines) != expected:
            raise PublishError("checksum evidence must match exactly both signed binary hashes and filenames")
    return {"manifest": manifest, "manifest_bytes": data, "files": files}


def _restricted_url(url):
    parts = urlsplit(url)
    base = urlsplit(BASE_URL)
    if parts.scheme != "https" or parts.netloc != base.netloc or parts.query or parts.fragment:
        raise PublishError("URL must stay on the fixed HTTPS distribution host")
    if not parts.path.startswith(base.path):
        raise PublishError("URL must stay in the fixed distribution directory")
    filename = parts.path[len(base.path):]
    if not re.fullmatch(r"[A-Za-z0-9_.-]+", filename) or ".." in filename:
        raise PublishError("unsafe distribution URL path")
    return parts


class HTTPTransport:
    """HTTPS with an overall deadline and bounded, incremental response reads."""
    def request(self, method, url, limit, timeout, body=None, headers=None, missing_ok=False):
        # Socket timeouts measure inactivity. A separate process is required to
        # bound DNS, TLS, trickled status/headers and HTTP chunk framing as well.
        # Killing it also prevents a timed-out upload from continuing later.
        deadline = time.monotonic() + timeout
        _restricted_url(url)
        arguments = {"method": method, "url": url, "limit": limit, "timeout": timeout,
                     "headers": headers, "missing_ok": missing_ok, "body_size": len(body) if body else 0}
        request = json.dumps(arguments).encode("utf-8") + b"\n" + (body or b"")
        try:
            with subprocess.Popen(self._worker_command(), stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                  stderr=subprocess.DEVNULL) as worker:
                try:
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise subprocess.TimeoutExpired(worker.args, timeout)
                    output, _ = worker.communicate(request, timeout=remaining)
                except subprocess.TimeoutExpired as error:
                    worker.kill()
                    worker.communicate()
                    raise PublishError("HTTP deadline exceeded") from error
                if time.monotonic() > deadline:
                    raise PublishError("HTTP deadline exceeded")
                status, separator, result = output.partition(b"\n")
                if not separator or status == b"error" or worker.returncode != 0:
                    message = result.decode("utf-8", errors="replace") if status == b"error" else "HTTP worker failed"
                    raise PublishError(message)
                if status == b"missing":
                    return None
                if status != b"ok" or len(result) > limit:
                    raise PublishError("invalid or oversized HTTP worker response")
                return result
        except OSError as error:
            raise PublishError("cannot start HTTP deadline worker") from error

    @staticmethod
    def _worker_command():
        return [sys.executable, "-c", 'import runpy,sys;runpy.run_path(sys.argv[1])["_http_worker"]()',
                str(Path(__file__).resolve())]

    def _request(self, method, url, limit, timeout, body=None, headers=None, missing_ok=False):
        deadline = time.monotonic() + timeout
        for redirects in range(6):
            parts = _restricted_url(url)
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise PublishError("HTTP deadline exceeded")
            connection = http.client.HTTPSConnection(parts.hostname, timeout=remaining)
            try:
                connection.request(method, parts.path, body=body, headers=headers or {})
                self._remaining(connection, deadline)
                response = connection.getresponse()
                self._remaining(connection, deadline, response)
                if response.status in (301, 302, 303, 307, 308):
                    if method != "GET" or redirects == 5:
                        raise PublishError("upload redirect or too many GET redirects")
                    location = response.getheader("Location")
                    if not location:
                        raise PublishError("redirect missing Location")
                    # Check before opening the next connection, including a downgrade.
                    url = urljoin(url, location)
                    _restricted_url(url)
                    continue
                if response.status == 404 and missing_ok:
                    return None
                if response.status not in ((200,) if method == "GET" else (200, 201, 204)):
                    raise PublishError("HTTP " + str(response.status) + " for " + method + " " + parts.path)
                declared = response.getheader("Content-Length")
                if declared is not None:
                    try:
                        length = int(declared)
                    except ValueError as error:
                        raise PublishError("invalid HTTP Content-Length") from error
                    if length < 0 or length > limit:
                        raise PublishError("HTTP response exceeds size limit")
                result = bytearray()
                while True:
                    self._remaining(connection, deadline, response)
                    chunk = response.read1(min(65536, limit + 1 - len(result)))
                    if time.monotonic() > deadline:
                        raise PublishError("HTTP deadline exceeded")
                    if not chunk:
                        break
                    result.extend(chunk)
                    if len(result) > limit:
                        raise PublishError("HTTP response exceeds size limit")
                return bytes(result)
            except (OSError, http.client.HTTPException) as error:
                raise PublishError("HTTP transport failed: " + method + " " + parts.path +
                                   ": " + type(error).__name__ + ": " + str(error)) from error
            finally:
                connection.close()
        raise PublishError("too many redirects")

    @staticmethod
    def _remaining(connection, deadline, response=None):
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise PublishError("HTTP deadline exceeded")
        stream = connection.sock
        # HTTP/1.0 and Connection: close retain the socket in the response reader.
        if stream is None and response is not None and response.fp is not None:
            stream = getattr(getattr(response.fp, "raw", None), "_sock", None)
        if stream is not None:
            stream.settimeout(remaining)

    def get(self, name, limit, timeout, missing_ok=False):
        return self.request("GET", BASE_URL + name, limit, timeout, missing_ok=missing_ok)

    def upload(self, name, data, timeout):
        url = BASE_URL + quote(name, safe="")
        _restricted_url(url)
        boundary = "boops-" + secrets.token_hex(24)
        body = ("--" + boundary + '\r\nContent-Disposition: form-data; name="blob"; filename="' + name +
                '"\r\nContent-Type: application/octet-stream\r\n\r\n').encode("ascii")
        body += data + ("\r\n--" + boundary + "--\r\n").encode("ascii")
        self.request("POST", url, MANIFEST_LIMIT, timeout, body=body,
                     headers={"Content-Type": "multipart/form-data; boundary=" + boundary})


def _http_worker():
    """Private pipe protocol used only by the supervising request process."""
    arguments = json.loads(sys.stdin.buffer.readline(16384))
    body_size = arguments.pop("body_size")
    body = sys.stdin.buffer.read(body_size) if body_size else None
    try:
        result = HTTPTransport()._request(body=body, **arguments)
        sys.stdout.buffer.write(b"missing\n" if result is None else b"ok\n" + result)
    except PublishError as error:
        sys.stdout.buffer.write(b"error\n" + str(error).encode("utf-8"))
        sys.exit(1)


def publish_release(release):
    transport = HTTPTransport()
    manifest = release["manifest"]
    files = release["files"]
    current = transport.get("latest.json", MANIFEST_LIMIT, 15, missing_ok=True)
    if current is not None:
        current_manifest = verify_manifest(current)
        if tuple(map(int, current_manifest["version"].split("."))) > tuple(map(int, manifest["version"].split("."))):
            raise PublishError("existing latest version is newer; refusing downgrade")
    existing = {}
    # Check both immutable paths before any POST. There is no delete operation.
    for artifact in manifest["artifacts"].values():
        name = artifact["path"]
        remote = transport.get(name, artifact["size"], 120, missing_ok=True)
        if remote is not None and not _matches(remote, artifact):
            raise PublishError("existing versioned binary differs; refusing overwrite: " + name)
        existing[name] = remote is not None
    for artifact in manifest["artifacts"].values():
        name = artifact["path"]
        if not existing[name]:
            transport.upload(name, files[name], 120)
        if not _matches(transport.get(name, artifact["size"], 120), artifact):
            raise PublishError("binary readback size/SHA-256 mismatch: " + name)
    for name in BOOTSTRAP_NAMES:
        if name in files:
            transport.upload(name, files[name], 15)
            if transport.get(name, MANIFEST_LIMIT, 15) != files[name]:
                raise PublishError("bootstrap readback mismatch: " + name)
    transport.upload("latest.json", release["manifest_bytes"], 15)
    readback = transport.get("latest.json", MANIFEST_LIMIT, 15)
    if readback != release["manifest_bytes"]:
        raise PublishError("published manifest readback mismatch")
    verify_manifest(readback)


def main(arguments=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dir", type=Path, required=True, help="directory containing signed release files")
    parser.add_argument("--publish", action="store_true", help="upload verified files to the fixed distribution endpoint")
    args = parser.parse_args(arguments)
    try:
        release = validate_release(args.dir)
        if args.publish:
            publish_release(release)
        print(("Published and read back " if args.publish else "Verified locally ") + release["manifest"]["version"])
        for platform, artifact in release["manifest"]["artifacts"].items():
            print(platform + " " + str(artifact["size"]) + " " + artifact["sha256"])
        return 0
    except PublishError as error:
        print("Release verification/publication failed: " + str(error), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
