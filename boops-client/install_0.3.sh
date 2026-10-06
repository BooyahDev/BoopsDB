#!/usr/bin/env bash
# Signed first migration for Linux amd64/arm64. Existing config/state are retained.
set -euo pipefail
umask 077
base_url='https://file.booyah.dev/BoopsDB-Client/'
root=${BOOPS_ROOT:-}
if [[ -n "$root" && "$root" != /* ]]; then echo 'BOOPS_ROOT must be an absolute path.' >&2;exit 1;fi
if (( $# > 1 ));then echo 'Usage: install_0.3.sh [existing-machine-id]' >&2;exit 1;fi
for command in id uname python3 openssl curl systemctl flock mktemp mkdir chmod cp mv rm cat dirname;do
    command -v "$command" >/dev/null || { echo "Required command not found: $command" >&2;exit 1; }
done
[[ $(id -u) = 0 ]] || { echo 'Run this installer as root.' >&2;exit 1; }
[[ $(uname -s) = Linux ]] || { echo 'This installer supports Linux only.' >&2;exit 1; }
python3 -c 'import base64,fcntl,hashlib,json,pathlib,subprocess,sys; assert sys.version_info >= (3,6)'
openssl_version=$(openssl version)
verify_libcrypto() {
    python3 - "$1" "$2" "$3" <<'PY'
import base64,ctypes,ctypes.util,pathlib,sys

def fail(message):
    raise SystemExit(message)

lines=pathlib.Path(sys.argv[1]).read_text().splitlines()
if len(lines)<3 or lines[0]!='-----BEGIN PUBLIC KEY-----' or lines[-1]!='-----END PUBLIC KEY-----':
    fail('Invalid embedded public key format')
try:
    der=base64.b64decode(''.join(lines[1:-1]).encode('ascii'),validate=True)
except (ValueError,UnicodeEncodeError):
    fail('Invalid embedded public key encoding')
prefix=bytes.fromhex('302a300506032b6570032100')
if len(der)!=len(prefix)+32 or der[:len(prefix)]!=prefix:
    fail('Embedded public key is not an Ed25519 SubjectPublicKeyInfo')
raw=der[len(prefix):]
payload=pathlib.Path(sys.argv[2]).read_bytes()
signature=pathlib.Path(sys.argv[3]).read_bytes()
if len(signature)!=64:
    fail('Invalid Ed25519 signature length')
found=ctypes.util.find_library('crypto')
candidates=[]
if found:
    candidates.append(found)
candidates.extend(('libcrypto.so.3','libcrypto.so.1.1','libcrypto.so','libcrypto.dylib'))
lib=None
for candidate in candidates:
    try:
        lib=ctypes.CDLL(candidate)
        break
    except OSError:
        pass
if lib is None:
    fail('No usable system libcrypto was found')
try:
    lib.EVP_PKEY_new_raw_public_key.argtypes=[ctypes.c_int,ctypes.c_void_p,ctypes.POINTER(ctypes.c_ubyte),ctypes.c_size_t]
    lib.EVP_PKEY_new_raw_public_key.restype=ctypes.c_void_p
    lib.EVP_PKEY_free.argtypes=[ctypes.c_void_p]
    lib.EVP_PKEY_free.restype=None
    lib.EVP_MD_CTX_new.argtypes=[]
    lib.EVP_MD_CTX_new.restype=ctypes.c_void_p
    lib.EVP_MD_CTX_free.argtypes=[ctypes.c_void_p]
    lib.EVP_MD_CTX_free.restype=None
    lib.EVP_DigestVerifyInit.argtypes=[ctypes.c_void_p,ctypes.c_void_p,ctypes.c_void_p,ctypes.c_void_p,ctypes.c_void_p]
    lib.EVP_DigestVerifyInit.restype=ctypes.c_int
    lib.EVP_DigestVerify.argtypes=[ctypes.c_void_p,ctypes.c_void_p,ctypes.c_size_t,ctypes.c_void_p,ctypes.c_size_t]
    lib.EVP_DigestVerify.restype=ctypes.c_int
except AttributeError:
    fail('System libcrypto lacks the Ed25519 EVP verification API')
byte_type=ctypes.c_ubyte
raw_buffer=(byte_type*len(raw)).from_buffer_copy(raw)
signature_buffer=(byte_type*len(signature)).from_buffer_copy(signature)
payload_buffer=None if not payload else (byte_type*len(payload)).from_buffer_copy(payload)
pkey=None
ctx=None
try:
    pkey=lib.EVP_PKEY_new_raw_public_key(1087,None,raw_buffer,len(raw))
    if not pkey:
        fail('System libcrypto could not load the Ed25519 public key')
    ctx=lib.EVP_MD_CTX_new()
    if not ctx:
        fail('System libcrypto could not create a verification context')
    if lib.EVP_DigestVerifyInit(ctx,None,None,None,pkey)!=1:
        fail('System libcrypto could not initialize Ed25519 verification')
    if lib.EVP_DigestVerify(ctx,signature_buffer,len(signature),payload_buffer,len(payload))!=1:
        fail('Ed25519 signature verification failed')
finally:
    if ctx:
        lib.EVP_MD_CTX_free(ctx)
    if pkey:
        lib.EVP_PKEY_free(pkey)
PY
}
crypto_probe=$(mktemp -d)
trap 'rm -rf "$crypto_probe"' EXIT
# This independent RFC vector probes the local verifier; it is not a trust key.
python3 - "$crypto_probe" <<'PY'
import pathlib,sys
p=pathlib.Path(sys.argv[1])
p.joinpath('probe-key.pem').write_text('''-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEA11qYAYKxCrfVS/7TyWQHOg7hcvPapiMlrwIaaPcHURo=
-----END PUBLIC KEY-----
''')
p.joinpath('probe-payload').write_bytes(b'boops installer crypto probe')
p.joinpath('probe-signature').write_bytes(bytes.fromhex('b09e6c6eda77627c954ab91f7b1438714f613307655489acd3203a23b861ea272b8f89021909bd508cf1a27fee5b600145002992e2c5a9fde24fec9ee48ad40c'))
PY
if [[ "$openssl_version" =~ ^OpenSSL\ ([3-9]|[1-9][0-9]+)\. ]];then
    verify_backend=openssl
    if ! openssl pkeyutl -verify -rawin -pubin -inkey "$crypto_probe/probe-key.pem" -in "$crypto_probe/probe-payload" -sigfile "$crypto_probe/probe-signature" >/dev/null 2>&1;then
        echo 'OpenSSL Ed25519 verification capability is unavailable.' >&2
        exit 1
    fi
else
    verify_backend=libcrypto
    verify_libcrypto "$crypto_probe/probe-key.pem" "$crypto_probe/probe-payload" "$crypto_probe/probe-signature"
fi
rm -rf "$crypto_probe"
trap - EXIT
case $(uname -m) in x86_64) arch=amd64;;aarch64|arm64) arch=arm64;;*) echo 'Unsupported CPU architecture.' >&2;exit 1;;esac
binary_dir="$root/usr/local/bin"
config_dir="$root/etc/boops"
unit_dir="$root/etc/systemd/system"
binary="$binary_dir/boops"
config="$config_dir/config.json"
existing_config=false
machine_id=${1:-}
if [[ -e "$config" ]];then
    existing_config=true
    python3 - "$config" <<'PY'
import json,sys
cfg=json.load(open(sys.argv[1]));assert isinstance(cfg,dict) and isinstance(cfg.get('id'),str) and cfg['id'], 'Existing config must contain a machine ID.'
assert 'auto_update' not in cfg or type(cfg['auto_update']) is bool, 'auto_update must be boolean.'
PY
else
    if [[ -z "$machine_id" ]];then read -rp 'Enter the machine ID already created in WebUI: ' machine_id;fi
    [[ -n "$machine_id" ]] || { echo 'A machine ID is required before enabling the timer.' >&2;exit 1; }
fi
mkdir -p "$binary_dir" "$config_dir" "$unit_dir"
exec 9>"$config_dir/update.lock"
flock -n 9 || { echo 'Another update is running. Retry after it finishes.' >&2;exit 1; }
staging=$(mktemp -d "$binary_dir/.boops-install.XXXXXX")
trap 'rm -rf "$staging"' EXIT
paths=("$binary" "$binary.previous" "$unit_dir/boops.service" "$unit_dir/boops.timer" "$config")
for index in "${!paths[@]}";do
    if [[ -e "${paths[$index]}" ]];then cp -p "${paths[$index]}" "$staging/backup-$index";fi
done
unit_present() {
    local load_state
    if ! load_state=$(systemctl show --property=LoadState --value "$1");then
        if [[ "$load_state" != not-found ]];then echo "Could not inspect unit $1." >&2;return 1;fi
    fi
    if [[ -z "$load_state" ]];then echo "Unit $1 returned no load state." >&2;return 1;fi
    if [[ "$load_state" = not-found ]];then printf 'false\n';else printf 'true\n';fi
}
timer_present=$(unit_present boops.timer)
service_present=$(unit_present boops.service)
timer_enabled=false;timer_active=false;service_active=false
if "$timer_present";then
    if systemctl is-enabled --quiet boops.timer;then timer_enabled=true;fi
    if systemctl is-active --quiet boops.timer;then timer_active=true;fi
fi
if "$service_present";then
    service_state=$(systemctl show --property=ActiveState --value boops.service)
    case "$service_state" in active|activating|reloading) service_active=true;;esac
fi
committed=false
stopped=false
restore_timer() {
    if ! "${1:-true}";then return 0;fi
    local failed=0
    if "$timer_enabled";then systemctl enable boops.timer || failed=1;else systemctl disable boops.timer || failed=1;fi
    if "$timer_active";then systemctl start boops.timer || failed=1;else systemctl stop boops.timer || failed=1;fi
    return "$failed"
}
finish() {
    local status=$?
    local keep_backup=false
    trap - EXIT
    if ! "$committed" && "$stopped";then
        set +e
        local rollback_failed=false
        if current_timer=$(unit_present boops.timer);then
            if "$current_timer";then
                systemctl stop boops.timer || rollback_failed=true
                if ! "$timer_present";then systemctl disable boops.timer || rollback_failed=true;fi
            fi
        else rollback_failed=true;fi
        if current_service=$(unit_present boops.service);then
            if "$current_service";then systemctl stop boops.service || rollback_failed=true;fi
        else rollback_failed=true;fi
        for index in "${!paths[@]}";do
            if [[ -f "$staging/backup-$index" ]];then
                target="${paths[$index]}"
                restore=$(mktemp "$(dirname "$target")/.boops-restore.XXXXXX")
                if [[ -n "$restore" ]] && cp -p "$staging/backup-$index" "$restore" && mv -f "$restore" "$target";then :;else rollback_failed=true;fi
            else
                rm -f "${paths[$index]}" || rollback_failed=true
            fi
        done
        systemctl daemon-reload || rollback_failed=true
        if "$service_active";then systemctl start boops.service || rollback_failed=true;fi
        restore_timer "$timer_present" || rollback_failed=true
        if "$rollback_failed";then
            keep_backup=true
            echo "Installation failed; restoration also reported an error. Recovery copies remain at $staging. Inspect the service and timer before retrying." >&2
        fi
    fi
    if ! "$keep_backup";then rm -rf "$staging";fi
    exit "$status"
}
trap finish EXIT
stopped=true
if "$timer_present";then systemctl stop boops.timer;fi
if "$service_present";then systemctl stop boops.service;fi
# This fixed SPKI is the sole release trust root; no manifest key is read.
cat > "$staging/public-key.pem" <<'PEM'
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAEnduz++M/m77CizmdKHP/Jzbh8CFMRJQFN+jGrTqBjM=
-----END PUBLIC KEY-----
PEM
# Stream into a bounded writer; even a chunked response cannot exceed the limit.
download() {
    local name=$1 destination=$2 limit=$3 seconds=$4
    curl --fail --silent --show-error --proto '=https' --tlsv1.2 --max-redirs 0 --max-time "$seconds" --dump-header "$staging/headers" "$base_url$name" |
        python3 -c 'import sys
out=open(sys.argv[1],"wb");limit=int(sys.argv[2]);size=0
while True:
 chunk=sys.stdin.buffer.read(65536)
 if not chunk:break
 size+=len(chunk)
 if size>limit:raise SystemExit("Download exceeds size limit")
 out.write(chunk)
out.close()' "$destination" "$limit"
    python3 - "$staging/headers" <<'PY'
import re,sys
statuses=re.findall(r'^HTTP/\S+\s+(\d+)',open(sys.argv[1]).read(),re.M)
assert statuses and 200 <= int(statuses[-1]) < 300, 'Download requires HTTP success and redirects are not accepted.'
PY
}
download latest.json "$staging/latest.json" 1048576 15
python3 - "$staging" <<'PY'
import base64,json,pathlib,sys
p=pathlib.Path(sys.argv[1]);e=json.loads((p/'latest.json').read_bytes())
assert isinstance(e,dict) and isinstance(e.get('payload'),str) and isinstance(e.get('signature'),str), 'Invalid signed envelope'
payload=base64.b64decode(e['payload'],validate=True);signature=base64.b64decode(e['signature'],validate=True)
assert len(signature)==64, 'Invalid Ed25519 signature length'
(p/'payload.json').write_bytes(payload);(p/'signature.bin').write_bytes(signature)
PY
if [[ "$verify_backend" = openssl ]];then
    openssl pkeyutl -verify -rawin -pubin -inkey "$staging/public-key.pem" -in "$staging/payload.json" -sigfile "$staging/signature.bin" >/dev/null
else
    verify_libcrypto "$staging/public-key.pem" "$staging/payload.json" "$staging/signature.bin"
fi
# Only use manifest fields after signature verification.
python3 - "$staging" "$arch" <<'PY'
import json,pathlib,re,sys
p=pathlib.Path(sys.argv[1]);m=json.loads((p/'payload.json').read_bytes())
assert isinstance(m,dict) and type(m.get('schema')) is int and m['schema']==1
version=m.get('version');assert isinstance(version,str) and re.fullmatch(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)',version), 'Invalid version'
artifacts=m.get('artifacts');assert isinstance(artifacts,dict)
for arch in ('amd64','arm64'):
 a=artifacts.get('linux/'+arch);assert isinstance(a,dict)
 assert a.get('path')=='boops_'+version+'_'+arch+'.binary', 'Unsafe or unexpected artifact filename'
 assert type(a.get('size')) is int and 0<a['size']<=67108864, 'Invalid binary size'
 assert isinstance(a.get('sha256'),str) and re.fullmatch('[0-9a-f]{64}',a['sha256']), 'Invalid SHA-256'
a=artifacts['linux/'+sys.argv[2]]
(p/'version').write_text(version);(p/'artifact').write_text(a['path'])
PY
new_version=$(<"$staging/version")
artifact=$(<"$staging/artifact")
download "$artifact" "$staging/new-boops" 67108864 120
python3 - "$staging" "$arch" <<'PY'
import hashlib,json,pathlib,sys
p=pathlib.Path(sys.argv[1]);a=json.loads((p/'payload.json').read_bytes())['artifacts']['linux/'+sys.argv[2]];b=(p/'new-boops').read_bytes()
assert len(b)==a['size'] and hashlib.sha256(b).hexdigest()==a['sha256'], 'Binary size or SHA-256 mismatch'
PY
chmod 0755 "$staging/new-boops"
python3 - "$staging/new-boops" "$new_version" <<'PY'
import subprocess,sys
result=subprocess.run([sys.argv[1],'version'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=20,check=True)
assert result.stdout.decode().strip()==sys.argv[2], 'Binary version probe mismatch'
PY
cat > "$staging/boops.service" <<'UNIT'
[Unit]
Description=Boops Client Service
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/boops sync
User=root
Group=root
WorkingDirectory=/etc/boops
UNIT
cat > "$staging/boops.timer" <<'UNIT'
[Unit]
Description=Runs Boops Client every minute

[Timer]
OnBootSec=1min
OnUnitActiveSec=1min
AccuracySec=1s
Unit=boops.service

[Install]
WantedBy=timers.target
UNIT
chmod 0644 "$staging/boops.service" "$staging/boops.timer"
if [[ -e "$binary" ]];then
    cp -p "$binary" "$staging/previous-boops"
    mv -f "$staging/previous-boops" "$binary.previous"
fi
mv -f "$staging/new-boops" "$binary"
# Stage each unit in its destination directory so replacement never crosses devices.
for unit in boops.service boops.timer;do
    temp_unit=$(mktemp "$unit_dir/.$unit.XXXXXX")
    cp -p "$staging/$unit" "$temp_unit"
    mv -f "$temp_unit" "$unit_dir/$unit"
done
systemctl daemon-reload
if "$existing_config";then
    restore_timer
else
    "$binary" regist "$machine_id"
    systemctl enable boops.timer
    systemctl start boops.timer
fi
committed=true
echo "Installed Boops Client $new_version. Existing config and network state were preserved."
