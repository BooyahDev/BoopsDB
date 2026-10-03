"""Real local HTTP fixture; no distribution endpoint is contacted."""
import base64
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from email.parser import BytesParser
from email.policy import default
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "publish.py"


class PublishTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.keys = tempfile.TemporaryDirectory()
        cls.keydir = Path(cls.keys.name)
        for name in ("trusted", "other"):
            subprocess.run(["openssl", "genpkey", "-algorithm", "ED25519", "-out",
                            str(cls.keydir / (name + ".key"))], check=True, capture_output=True)
            subprocess.run(["openssl", "pkey", "-in", str(cls.keydir / (name + ".key")),
                            "-pubout", "-out", str(cls.keydir / (name + ".pem"))],
                           check=True, capture_output=True)

    @classmethod
    def tearDownClass(cls):
        cls.keys.cleanup()

    def setUp(self):
        # Missing implementation is an assertion failure, not an import error.
        self.assertTrue(SCRIPT.exists(), "signed release publisher has not been implemented")
        spec = importlib.util.spec_from_file_location("boops_publish", SCRIPT)
        self.publisher = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.publisher)
        self.trust = patch.object(self.publisher, "TRUSTED_PUBLIC_KEY", self.keydir / "trusted.pem")
        self.trust.start()
        self.addCleanup(self.trust.stop)
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.binaries = {"boops_0.3.0_amd64.binary": b"amd64 fixture bytes",
                         "boops_0.3.0_arm64.binary": b"arm64 fixture bytes"}
        for name, data in self.binaries.items():
            (self.directory / name).write_bytes(data)
        self.payload = {"schema": 1, "version": "0.3.0", "artifacts": {
            "linux/" + arch: {"path": "boops_0.3.0_" + arch + ".binary",
                              "size": len(self.binaries["boops_0.3.0_" + arch + ".binary"]),
                              "sha256": hashlib.sha256(self.binaries["boops_0.3.0_" + arch + ".binary"]).hexdigest()}
            for arch in ("amd64", "arm64")}}
        self.sign()
        self.files = {"boops_0.1_amd64.binary": b"old release", "install_0.2.sh": b"old installer"}
        self.events = []
        self.post_paths = []
        self.corrupt = None
        self.corrupt_same_size = False
        self.redirect = None
        self.post_status = 201
        self.omit_length = False
        self.slow = False
        self.post_redirect = False
        self.slow_headers = False
        self.slow_chunk_framing = False
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                name = self.path.rsplit("/", 1)[-1]
                owner.events.append(("GET", name))
                if owner.slow_headers or owner.slow_chunk_framing:
                    try:
                        if owner.slow_headers:
                            self.wfile.write(b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nX: ")
                            trickle, final = b"abcd\r\n\r\n", b""
                        else:
                            self.wfile.write(b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n")
                            trickle, final = b"0001\r\n", b"x\r\n0\r\n\r\n"
                        self.wfile.flush()
                        for byte in trickle:
                            time.sleep(0.04)
                            self.wfile.write(bytes([byte]))
                            self.wfile.flush()
                        self.wfile.write(final)
                    except (BrokenPipeError, ConnectionResetError):
                        pass
                    return
                location = owner.redirect.get(name) if isinstance(owner.redirect, dict) else owner.redirect
                if location:
                    self.send_response(302)
                    self.send_header("Location", location)
                    self.end_headers()
                    return
                if name not in owner.files:
                    self.send_response(404)
                    self.end_headers()
                    return
                data = owner.files[name]
                if owner.corrupt == name:
                    data = b"x" + data[1:] if owner.corrupt_same_size else data + b"corrupt"
                self.send_response(200)
                if not owner.omit_length:
                    self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                if owner.slow:
                    self.wfile.write(data[:1])
                    self.wfile.flush()
                    time.sleep(0.07)
                    self.wfile.write(data[1:2])
                    self.wfile.flush()
                    time.sleep(0.25)
                    try:
                        self.wfile.write(data[2:])
                    except (BrokenPipeError, ConnectionResetError):
                        pass
                    return
                self.wfile.write(data)

            def do_POST(self):
                raw = self.rfile.read(int(self.headers["Content-Length"]))
                message = BytesParser(policy=default).parsebytes(
                    ("Content-Type: " + self.headers["Content-Type"] + "\r\n\r\n").encode() + raw)
                parts = list(message.iter_parts())
                if len(parts) != 1 or parts[0].get_param("name", header="content-disposition") != "blob":
                    self.send_response(400)
                    self.end_headers()
                    return
                name = parts[0].get_filename()
                owner.events.append(("POST", name))
                owner.post_paths.append(self.path)
                if owner.post_redirect:
                    self.send_response(307)
                    self.send_header("Location", "https://file.booyah.dev/BoopsDB-Client/")
                    self.end_headers()
                    return
                # Actual distribution frontend POSTs to the encoded file path,
                # while the multipart field remains named "blob".
                if self.path != "/BoopsDB-Client/" + name:
                    self.send_response(400)
                    self.end_headers()
                    return
                if owner.post_status in (200, 201, 204):
                    owner.files[name] = parts[0].get_payload(decode=True)
                self.send_response(owner.post_status)
                self.end_headers()

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.addCleanup(self.stop_server)
        port = self.server.server_port
        # Spawn the actual production worker, substituting only the connection
        # to a loopback HTTP socket. URL restrictions and parsing stay real.
        worker_code = ('import http.client,runpy,sys;ns=runpy.run_path(sys.argv[1]);'
                       'http.client.HTTPSConnection=lambda host,timeout:'
                       'http.client.HTTPConnection("127.0.0.1",int(sys.argv[2]),timeout=timeout);'
                       'ns["_http_worker"]()')
        self.worker_code = worker_code
        self.fixture_port = port
        self.connection_patch = patch.object(self.publisher.HTTPTransport, "_worker_command",
                                            return_value=[sys.executable, "-c", worker_code, str(SCRIPT), str(port)])
        self.connection_patch.start()
        self.addCleanup(self.connection_patch.stop)

    def stop_server(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def sign(self, raw=None, key="trusted", envelope_extra=None):
        raw = raw if raw is not None else json.dumps(self.payload, separators=(",", ":")).encode()
        (self.directory / "payload.tmp").write_bytes(raw)
        subprocess.run(["openssl", "pkeyutl", "-sign", "-rawin", "-inkey",
                        str(self.keydir / (key + ".key")), "-in", str(self.directory / "payload.tmp"),
                        "-out", str(self.directory / "signature.tmp")], check=True, capture_output=True)
        envelope = {"payload": base64.b64encode(raw).decode(),
                    "signature": base64.b64encode((self.directory / "signature.tmp").read_bytes()).decode()}
        envelope.update(envelope_extra or {})
        (self.directory / "latest.json").write_text(json.dumps(envelope))
        (self.directory / "payload.tmp").unlink()
        (self.directory / "signature.tmp").unlink()

    def publish(self):
        return self.publisher.publish_release(self.publisher.validate_release(self.directory))

    def uploads(self):
        return [name for method, name in self.events if method == "POST"]

    def test_manifest_is_last_after_both_binary_readbacks(self):
        self.publish()
        self.assertEqual(self.uploads()[-1], "latest.json")
        manifest_index = self.events.index(("POST", "latest.json"))
        for name, expected in self.binaries.items():
            upload_index = self.events.index(("POST", name))
            self.assertIn(("GET", name), self.events[upload_index + 1:manifest_index])
            self.assertEqual(self.files[name], expected)
        self.assertEqual(self.files["boops_0.1_amd64.binary"], b"old release")
        self.assertEqual(self.files["install_0.2.sh"], b"old installer")

    def test_cli_uploads_to_each_fixed_child_filename(self):
        self.assertEqual(self.publisher.main(["--dir", str(self.directory), "--publish"]), 0)
        self.assertEqual(self.post_paths, ["/BoopsDB-Client/boops_0.3.0_amd64.binary",
                                           "/BoopsDB-Client/boops_0.3.0_arm64.binary",
                                           "/BoopsDB-Client/latest.json"])

    def test_upload_rejects_unsafe_child_names_before_http(self):
        for name in ("../private-key.pem", "https://evil.test/x", "install.sh?x=1", "", "別名.sh"):
            with self.subTest(name=name):
                with self.assertRaises(self.publisher.PublishError):
                    self.publisher.HTTPTransport().upload(name, b"fixture", 15)
                self.assertEqual(self.events, [])

    def test_readback_mismatch_prevents_manifest_publish(self):
        self.corrupt = "boops_0.3.0_arm64.binary"
        self.corrupt_same_size = True
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertNotIn("latest.json", self.uploads())

    def test_existing_different_version_artifact_is_not_overwritten(self):
        name = "boops_0.3.0_arm64.binary"
        self.files[name] = b"different old bytes"
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.files[name], b"different old bytes")
        self.assertEqual(self.uploads(), [])

    def test_existing_newer_latest_prevents_all_uploads(self):
        original_manifest = (self.directory / "latest.json").read_bytes()
        self.payload["version"] = "0.4.0"
        for arch in ("amd64", "arm64"):
            self.payload["artifacts"]["linux/" + arch]["path"] = "boops_0.4.0_" + arch + ".binary"
        self.sign()
        self.files["latest.json"] = (self.directory / "latest.json").read_bytes()
        (self.directory / "latest.json").write_bytes(original_manifest)
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.uploads(), [])

    def test_invalid_existing_latest_stops_all_uploads(self):
        self.files["latest.json"] = b"untrusted remote manifest"
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.uploads(), [])

    def test_identical_existing_artifacts_are_skipped_and_readback(self):
        self.files.update(self.binaries)
        self.publish()
        self.assertEqual(self.uploads(), ["latest.json"])
        for name in self.binaries:
            self.assertIn(("GET", name), self.events)

    def test_signature_from_other_key_is_rejected_before_http(self):
        self.sign(key="other", envelope_extra={"public_key": (self.keydir / "other.pem").read_text()})
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.events, [])

    def test_wrong_signature_without_supplied_key_is_rejected(self):
        self.sign(key="other")
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.events, [])

    def test_local_hash_mismatch_prevents_all_http(self):
        (self.directory / "boops_0.3.0_arm64.binary").write_bytes(b"tampered")
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.events, [])

    def test_duplicate_json_keys_are_rejected_before_http(self):
        raw = json.dumps(self.payload).replace('"schema": 1', '"schema": 2, "schema": 1').encode()
        self.sign(raw=raw)
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.events, [])

    def test_version_components_match_go_uint64_range(self):
        self.payload["version"] = "18446744073709551616.0.0"
        for arch in ("amd64", "arm64"):
            self.payload["artifacts"]["linux/" + arch]["path"] = "boops_18446744073709551616.0.0_" + arch + ".binary"
        self.sign()
        with self.assertRaises(self.publisher.PublishError):
            self.publisher.verify_manifest((self.directory / "latest.json").read_bytes())

    def test_noncanonical_base64_signature_is_rejected(self):
        envelope = json.loads((self.directory / "latest.json").read_bytes())
        # For 64 bytes, the final Base64 digit has four unused bits. Changing
        # those bits decodes identically in permissive decoders, unlike Go Strict.
        alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
        signature = envelope["signature"]
        digit = alphabet.index(signature[-3])
        envelope["signature"] = signature[:-3] + alphabet[digit + 1] + "=="
        with self.assertRaises(self.publisher.PublishError):
            self.publisher.verify_manifest(json.dumps(envelope).encode())

    def test_signature_uses_raw_payload_without_reserializing(self):
        self.sign(raw=json.dumps(self.payload, indent=2).encode() + b"\n")
        self.publish()
        self.assertEqual(self.files["latest.json"], (self.directory / "latest.json").read_bytes())

    def test_invalid_signed_artifacts_are_rejected(self):
        for field, value in (("path", "../boops_0.3.0_amd64.binary"), ("size", True),
                             ("size", 67108865), ("sha256", "invalid")):
            with self.subTest(field=field, value=value):
                original = self.payload["artifacts"]["linux/amd64"][field]
                self.payload["artifacts"]["linux/amd64"][field] = value
                self.sign()
                with self.assertRaises(self.publisher.PublishError):
                    self.publish()
                self.payload["artifacts"]["linux/amd64"][field] = original
        self.assertEqual(self.events, [])

    def test_unknown_platform_is_rejected(self):
        self.payload["artifacts"]["linux/386"] = self.payload["artifacts"]["linux/amd64"]
        self.sign()
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.events, [])

    def test_only_allowlisted_bootstrap_files_are_uploaded(self):
        for name, data in (("install.sh", b"installer"), ("install_0.3.sh", b"installer"),
                           ("public-key.pem", (self.keydir / "trusted.pem").read_bytes()),
                           ("verification-ja.md", b"fixture evidence")):
            (self.directory / name).write_bytes(data)
        (self.directory / "private-key.pem").write_bytes(b"never publish")
        self.publish()
        self.assertNotIn("private-key.pem", self.uploads())
        self.assertEqual(self.uploads()[-1], "latest.json")
        self.assertEqual(len(self.uploads()), 7)

    def checksums(self):
        return "".join(self.payload["artifacts"]["linux/" + arch]["sha256"] +
                       "  boops_0.3.0_" + arch + ".binary\n" for arch in ("amd64", "arm64")).encode()

    def test_checksums_evidence_is_verified_uploaded_and_readback(self):
        data = self.checksums()
        (self.directory / "SHA256SUMS").write_bytes(data)
        self.publish()
        self.assertIn("SHA256SUMS", self.uploads())
        self.assertEqual(self.files["SHA256SUMS"], data)
        self.assertEqual(self.uploads()[-1], "latest.json")

    def test_invalid_checksum_evidence_stops_all_http(self):
        for data in (self.checksums().replace(b"boops_0.3.0_amd64.binary", b"../private-key.pem"),
                     b"0" * 64 + self.checksums()[64:],
                     self.checksums().splitlines(keepends=True)[0] * 2):
            with self.subTest(data=data):
                (self.directory / "SHA256SUMS").write_bytes(data)
                with self.assertRaises(self.publisher.PublishError):
                    self.publish()
                self.assertEqual(self.events, [])

    def test_installer_alias_mismatch_and_wrong_public_key_stop_all_http(self):
        (self.directory / "install.sh").write_bytes(b"alias only")
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        (self.directory / "install_0.3.sh").write_bytes(b"different")
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        (self.directory / "install.sh").unlink()
        (self.directory / "install_0.3.sh").unlink()
        (self.directory / "public-key.pem").write_bytes((self.keydir / "other.pem").read_bytes())
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.events, [])

    def test_upload_failure_prevents_manifest(self):
        self.post_status = 500
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertNotIn("latest.json", self.uploads())

    def test_redirect_restrictions_prevent_unsafe_fetch(self):
        for location in ("http://file.booyah.dev/BoopsDB-Client/evil", "https://evil.test/x",
                         "https://file.booyah.dev/other/file", "https://file.booyah.dev/BoopsDB-Client/../evil"):
            with self.subTest(location=location):
                self.redirect = location
                self.events.clear()
                with self.assertRaises(self.publisher.PublishError):
                    self.publish()
                self.assertEqual(len(self.events), 1)
                self.assertEqual(self.uploads(), [])

    def test_manifest_readback_mismatch_is_reported(self):
        self.corrupt = "latest.json"
        with self.assertRaises(self.publisher.PublishError):
            self.publish()

    def test_stream_without_content_length_still_enforces_size_limit(self):
        self.omit_length = True
        self.corrupt = "boops_0.3.0_amd64.binary"
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertNotIn("latest.json", self.uploads())

    def test_same_host_redirect_allowed_but_loop_is_bounded(self):
        name = "boops_0.3.0_amd64.binary"
        self.files[name] = self.binaries[name]
        self.files["alias.binary"] = self.binaries[name]
        self.redirect = {name: "/BoopsDB-Client/alias.binary"}
        data = self.publisher.HTTPTransport().get(name, 19, 15)
        self.assertEqual(data, self.binaries[name])
        self.assertEqual(self.events, [("GET", name), ("GET", "alias.binary")])
        self.events.clear()
        self.redirect = "/BoopsDB-Client/alias.binary"
        with self.assertRaises(self.publisher.PublishError):
            self.publisher.HTTPTransport().get(name, 19, 15)
        self.assertEqual(len(self.events), 6)

    def test_post_redirect_is_rejected_without_retry(self):
        self.post_redirect = True
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(len(self.uploads()), 1)
        self.assertNotIn("latest.json", self.uploads())

    def test_overall_deadline_applies_after_connection_close_header(self):
        # HTTP/1.0 clears connection.sock when headers arrive. A deadline must
        # still update the retained response socket before every incremental read.
        name = "boops_0.3.0_amd64.binary"
        self.files[name] = self.binaries[name]
        self.slow = True
        started = time.monotonic()
        with self.assertRaises(self.publisher.PublishError):
            self.publisher.HTTPTransport().get(name, 19, 0.1)
        self.assertLess(time.monotonic() - started, 0.145)
        self.assertEqual(self.events, [("GET", name)])

    def test_trickled_headers_cannot_return_missing_after_deadline(self):
        self.slow_headers = True
        started = time.monotonic()
        with self.assertRaises(self.publisher.PublishError):
            self.publisher.HTTPTransport().get("missing.binary", 19, 0.1, missing_ok=True)
        self.assertLess(time.monotonic() - started, 0.2)
        self.assertEqual(self.events, [("GET", "missing.binary")])

    def test_trickled_chunk_framing_cannot_extend_deadline(self):
        self.slow_chunk_framing = True
        started = time.monotonic()
        with self.assertRaises(self.publisher.PublishError):
            self.publisher.HTTPTransport().get("chunked.binary", 19, 0.1)
        self.assertLess(time.monotonic() - started, 0.2)
        self.assertEqual(self.events, [("GET", "chunked.binary")])

    def test_stalled_dns_is_killed_and_reaped_before_return(self):
        code = self.worker_code.replace('ns["_http_worker"]()',
                'import socket,time;resolve=socket.getaddrinfo;'
                'socket.getaddrinfo=lambda *a,**kw:(time.sleep(5),resolve(*a,**kw))[1];'
                'ns["_http_worker"]()')
        command = [sys.executable, "-c", code, str(SCRIPT), str(self.fixture_port)]
        spawned = []
        start_process = subprocess.Popen

        def track_process(*args, **kwargs):
            worker = start_process(*args, **kwargs)
            spawned.append(worker)
            return worker

        started = time.monotonic()
        with patch.object(self.publisher.HTTPTransport, "_worker_command", return_value=command), \
                patch.object(self.publisher.subprocess, "Popen", side_effect=track_process):
            with self.assertRaises(self.publisher.PublishError):
                self.publisher.HTTPTransport().get("missing.binary", 19, 0.1, missing_ok=True)
        self.assertLess(time.monotonic() - started, 0.2)
        self.assertEqual(len(spawned), 1)
        self.assertIsNotNone(spawned[0].poll())
        self.assertEqual(self.events, [])

    def test_default_cli_only_validates_without_http(self):
        self.assertEqual(self.publisher.main(["--dir", str(self.directory)]), 0)
        self.assertEqual(self.events, [])

    def test_oversized_manifest_and_symlink_are_rejected_locally(self):
        (self.directory / "latest.json").write_bytes(b"x" * (1048576 + 1))
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.sign()
        name = self.directory / "boops_0.3.0_amd64.binary"
        name.unlink()
        name.symlink_to(self.directory / "boops_0.3.0_arm64.binary")
        with self.assertRaises(self.publisher.PublishError):
            self.publish()
        self.assertEqual(self.events, [])


if __name__ == "__main__":
    unittest.main()
