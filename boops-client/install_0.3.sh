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
[[ "$openssl_version" =~ ^OpenSSL\ ([3-9]|[1-9][0-9]+)\. ]] || { echo 'OpenSSL 3.0 or newer is required for Ed25519 verification.' >&2;exit 1; }
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
timer_enabled=false;timer_active=false;service_active=false
if systemctl is-enabled --quiet boops.timer;then timer_enabled=true;fi
if systemctl is-active --quiet boops.timer;then timer_active=true;fi
if systemctl is-active --quiet boops.service;then service_active=true;fi
committed=false
stopped=false
restore_timer() {
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
        systemctl stop boops.timer || rollback_failed=true
        systemctl stop boops.service || rollback_failed=true
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
        restore_timer || rollback_failed=true
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
systemctl stop boops.timer
systemctl stop boops.service
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
openssl pkeyutl -verify -rawin -pubin -inkey "$staging/public-key.pem" -in "$staging/payload.json" -sigfile "$staging/signature.bin" >/dev/null
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
