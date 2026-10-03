#!/usr/bin/env bash
set -euo pipefail
script_dir=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
real_openssl=$(command -v openssl)
real_mv=$(command -v mv)
export REAL_OPENSSL="$real_openssl" REAL_MV="$real_mv"
mkdir -p "$work/bin" "$work/downloads"
"$real_openssl" genpkey -algorithm ED25519 -out "$work/key.pem" >/dev/null 2>&1
"$real_openssl" pkey -in "$work/key.pem" -pubout -out "$work/pub.pem" >/dev/null 2>&1
# Use a fixture trust root only in the temporary installer copy.
python3 - "$script_dir/install_0.3.sh" "$work/installer.sh" "$work/pub.pem" <<'PY'
import pathlib,re,sys
text=pathlib.Path(sys.argv[1]).read_text()
text,n=re.subn(r'-----BEGIN PUBLIC KEY-----\n.*?\n-----END PUBLIC KEY-----',pathlib.Path(sys.argv[3]).read_text().strip(),text,flags=re.S)
assert n==1
pathlib.Path(sys.argv[2]).write_text(text)
PY
cat > "$work/bin/id" <<'SH'
#!/bin/sh
printf '0\n'
SH
cat > "$work/bin/uname" <<'SH'
#!/bin/sh
if [ "$1" = -s ];then printf 'Linux\n';else printf 'x86_64\n';fi
SH
cat > "$work/bin/flock" <<'SH'
#!/bin/sh
exit 0
SH
cat > "$work/bin/openssl" <<'SH'
#!/bin/sh
if [ "$1" = version ] && [ "${OLD_OPENSSL:-0}" = 1 ]; then printf 'OpenSSL 1.1.1\n'; exit 0; fi
exec "$REAL_OPENSSL" "$@"
SH
cat > "$work/bin/systemctl" <<'PY'
#!/usr/bin/env python3
import json,os,pathlib,sys
args=[a for a in sys.argv[1:] if a!='--quiet']
path=pathlib.Path(os.environ['FIXTURE_STATE'])
s=json.loads(path.read_text())
with open(os.environ['FIXTURE_LOG'],'a') as f:f.write('systemctl '+' '.join(args)+'\n')
command=args[0];unit=args[-1]
exists=(pathlib.Path(os.environ['BOOPS_ROOT'])/'etc/systemd/system'/unit).is_file()
if command=='show':
 if '--property=LoadState' in args:print('loaded' if exists else 'not-found')
 elif '--property=ActiveState' in args:print(('active' if s['active'] else 'inactive') if unit=='boops.timer' else s.get('service_status','active' if s.get('service_active') else 'inactive'))
 else:sys.exit(1)
 sys.exit(0)
if command in ('start','stop','enable','disable') and not exists:sys.exit(5)
if command=='is-enabled':sys.exit(0 if s['enabled'] else 1)
if command=='is-active':sys.exit(0 if (s['active'] if unit=='boops.timer' else s.get('service_status','active' if s.get('service_active') else 'inactive')=='active') else 3)
if command=='daemon-reload' and os.environ.get('FAIL_RELOAD')=='1' and not s.get('failed_reload'):
 s['failed_reload']=True;path.write_text(json.dumps(s));sys.exit(1)
if command in ('enable','disable'):s['enabled']=command=='enable'
if command=='stop' and unit==os.environ.get('FAIL_STOP_UNIT'):sys.exit(1)
if command=='start' and unit=='boops.service' and s.get('failed_reload'):
 root=pathlib.Path(os.environ['BOOPS_ROOT'])
 assert (root/'usr/local/bin/boops').read_bytes()==pathlib.Path(os.environ['FIXTURE_BINARY_BEFORE']).read_bytes(), 'service resumed before old binary restoration'
 assert (root/'etc/systemd/system/boops.service').read_text()=='old service\n', 'service resumed before old service unit restoration'
 assert (root/'etc/systemd/system/boops.timer').read_text()=='old timer\n', 'service resumed before old timer unit restoration'
if command in ('start','stop'):
 s['active' if unit=='boops.timer' else 'service_active']=command=='start'
 if unit=='boops.service':s['service_status']='active' if command=='start' else 'inactive'
path.write_text(json.dumps(s))
PY
cat > "$work/bin/curl" <<'PY'
#!/usr/bin/env python3
import os,pathlib,sys
args=sys.argv[1:];url=args[-1]
headers=args[args.index('--dump-header')+1]
status='404' if os.environ.get('FAIL_HTTP')=='1' else '200'
pathlib.Path(headers).write_text('HTTP/1.1 '+status+' fixture\r\n\r\n')
with open(os.environ['FIXTURE_LOG'],'a') as f:f.write('curl '+url+'\n')
if status=='404':sys.exit(22)
name=url.rsplit('/',1)[1]
data=(pathlib.Path(os.environ['FIXTURE_DOWNLOADS'])/name).read_bytes()
try:sys.stdout.buffer.write(data);sys.stdout.buffer.flush()
except BrokenPipeError:sys.exit(23)
PY
cat > "$work/bin/mv" <<'SH'
#!/bin/sh
printf 'mv %s\n' "$*" >> "$FIXTURE_LOG"
exec "$REAL_MV" "$@"
SH
chmod +x "$work/bin/"*
export PATH="$work/bin:$PATH" FIXTURE_DOWNLOADS="$work/downloads"
make_release() {
    local output_version=${1:-0.3.0}
    cat > "$work/downloads/boops_0.3.0_amd64.binary" <<SH
#!/bin/sh
case "\$1" in
version) printf '%s\\n' '$output_version';;
regist) printf 'regist %s\\n' "\$2" >> "\$FIXTURE_LOG"; mkdir -p "\$BOOPS_ROOT/etc/boops"; printf '{"id":"%s"}' "\$2" > "\$BOOPS_ROOT/etc/boops/config.json"; [ "\${FAIL_REGISTRATION:-0}" != 1 ];;
*) exit 1;;
esac
SH
    python3 - "$work/downloads" <<'PY'
import hashlib,json,pathlib,sys
p=pathlib.Path(sys.argv[1]);binary=(p/'boops_0.3.0_amd64.binary').read_bytes()
a={'path':'boops_0.3.0_amd64.binary','size':len(binary),'sha256':hashlib.sha256(binary).hexdigest()}
(p/'payload.json').write_bytes(json.dumps({'schema':1,'version':'0.3.0','artifacts':{'linux/amd64':a,'linux/arm64':{**a,'path':'boops_0.3.0_arm64.binary'}}},separators=(',',':')).encode())
PY
    "$real_openssl" pkeyutl -sign -rawin -inkey "$work/key.pem" -in "$work/downloads/payload.json" -out "$work/signature.bin"
    python3 - "$work" <<'PY'
import base64,json,pathlib,sys
p=pathlib.Path(sys.argv[1]);(p/'downloads/latest.json').write_text(json.dumps({'payload':base64.b64encode((p/'downloads/payload.json').read_bytes()).decode(),'signature':base64.b64encode((p/'signature.bin').read_bytes()).decode()}))
PY
}
reset_fixture() {
    rm -rf "$work/root";mkdir -p "$work/root/usr/local/bin" "$work/root/etc/boops" "$work/root/etc/systemd/system"
    export BOOPS_ROOT="$work/root" FIXTURE_STATE="$work/state.json" FIXTURE_LOG="$work/log"
    export FIXTURE_BINARY_BEFORE="$work/binary.before"
    printf '{"enabled":%s,"active":%s,"service_active":false}' "$1" "$2" > "$FIXTURE_STATE"
    : > "$FIXTURE_LOG"
    printf '#!/bin/sh\nprintf "0.2.0\\n"\n' > "$BOOPS_ROOT/usr/local/bin/boops";chmod +x "$BOOPS_ROOT/usr/local/bin/boops"
    printf 'old service\n' > "$BOOPS_ROOT/etc/systemd/system/boops.service"
    printf 'old timer\n' > "$BOOPS_ROOT/etc/systemd/system/boops.timer"
    printf '{ "id": "existing-uuid", "auto_update":false, "unknown": [1,2] }\n' > "$BOOPS_ROOT/etc/boops/config.json"
    printf '{"hostname":"old","interfaces":[]}\n' > "$BOOPS_ROOT/etc/boops/machine_state.json"
    cp "$BOOPS_ROOT/etc/boops/config.json" "$work/config.before"
    cp "$BOOPS_ROOT/etc/boops/machine_state.json" "$work/state.before"
    cp "$BOOPS_ROOT/usr/local/bin/boops" "$work/binary.before"
    unset FAIL_RELOAD FAIL_HTTP OLD_OPENSSL FAIL_REGISTRATION FAIL_STOP_UNIT
    make_release
}
assert_preserved() {
    cmp "$work/config.before" "$BOOPS_ROOT/etc/boops/config.json"
    cmp "$work/state.before" "$BOOPS_ROOT/etc/boops/machine_state.json"
    ! rg -q '^regist ' "$FIXTURE_LOG"
    python3 - "$FIXTURE_STATE" "$1" "$2" <<'PY'
import json,sys
s=json.load(open(sys.argv[1]));assert s['enabled']==(sys.argv[2]=='true');assert s['active']==(sys.argv[3]=='true')
PY
}
assert_rollback() {
    assert_preserved "$1" "$2"
    cmp "$work/binary.before" "$BOOPS_ROOT/usr/local/bin/boops"
    test "$(cat "$BOOPS_ROOT/etc/systemd/system/boops.service")" = 'old service'
    test "$(cat "$BOOPS_ROOT/etc/systemd/system/boops.timer")" = 'old timer'
}
run_failure() {
    if bash "$work/installer.sh" > "$work/output" 2>&1;then cat "$work/output";echo 'installer unexpectedly succeeded' >&2;exit 1;fi
}
for status in 'true true' 'false false' 'false true';do
    read -r enabled active <<< "$status";reset_fixture "$enabled" "$active"
    bash "$work/installer.sh" > "$work/output" 2>&1
    assert_preserved "$enabled" "$active"
    test "$("$BOOPS_ROOT/usr/local/bin/boops" version)" = 0.3.0
    python3 - "$FIXTURE_LOG" <<'PY'
import sys
lines=open(sys.argv[1]).read().splitlines();replace=next(i for i,v in enumerate(lines) if v.startswith('mv ') and v.endswith('/usr/local/bin/boops'))
assert lines.index('systemctl stop boops.timer')<lines.index('systemctl stop boops.service')<replace
PY
    echo "PASS migration retains timer $status"
done
reset_fixture true true;OLD_OPENSSL=1;export OLD_OPENSSL;run_failure
assert_rollback true true
! rg -q '^systemctl stop ' "$FIXTURE_LOG"
echo 'PASS prerequisites checked before stopping'
for failure in signature hash probe http oversize reload;do
    reset_fixture true true
    case "$failure" in
    signature) python3 - "$work/downloads/latest.json" <<'PY'
import json,sys
p=sys.argv[1];e=json.load(open(p));e['signature']='A'*86+'==';open(p,'w').write(json.dumps(e))
PY
    ;;
    hash) printf '\ncorruption\n' >> "$work/downloads/boops_0.3.0_amd64.binary";;
    probe) make_release 0.9.9;;
    http) export FAIL_HTTP=1;;
    oversize) python3 - "$work/downloads/latest.json" <<'PY'
import sys
open(sys.argv[1],'wb').write(b'x'*(1024*1024+1))
PY
    ;;
    reload) export FAIL_RELOAD=1;;
    esac
    run_failure;assert_rollback true true
    echo "PASS rollback on $failure failure"
done
reset_fixture false false;rm "$BOOPS_ROOT/etc/boops/config.json"
bash "$work/installer.sh" new-machine > "$work/output" 2>&1
python3 - "$FIXTURE_STATE" "$BOOPS_ROOT/etc/boops/config.json" <<'PY'
import json,sys
s=json.load(open(sys.argv[1]));assert s['enabled'] and s['active'];assert json.load(open(sys.argv[2]))['id']=='new-machine'
PY
python3 - "$FIXTURE_LOG" <<'PY'
import sys
s=open(sys.argv[1]).read();assert s.index('regist new-machine')<s.index('systemctl enable boops.timer')<s.index('systemctl start boops.timer')
PY
echo 'PASS new registration precedes timer activation'
reset_fixture false false;rm "$BOOPS_ROOT/etc/boops/config.json";export FAIL_REGISTRATION=1
if bash "$work/installer.sh" new-machine > "$work/output" 2>&1;then exit 1;fi
test ! -e "$BOOPS_ROOT/etc/boops/config.json"
cmp "$work/binary.before" "$BOOPS_ROOT/usr/local/bin/boops"
echo 'PASS registration failure restores prior installation'
reset_fixture false false
rm "$BOOPS_ROOT/usr/local/bin/boops" "$BOOPS_ROOT/etc/systemd/system/boops.service" "$BOOPS_ROOT/etc/systemd/system/boops.timer" "$BOOPS_ROOT/etc/boops/config.json"
bash "$work/installer.sh" fresh-machine > "$work/output" 2>&1
python3 - "$FIXTURE_STATE" "$FIXTURE_LOG" "$BOOPS_ROOT/etc/boops/config.json" <<'PY'
import json,sys
s=json.load(open(sys.argv[1]));assert s['enabled'] and s['active']
log=open(sys.argv[2]).read();assert log.index('regist fresh-machine')<log.index('systemctl enable boops.timer')<log.index('systemctl start boops.timer')
assert 'systemctl stop boops.timer' not in log and 'systemctl stop boops.service' not in log
assert json.load(open(sys.argv[3]))['id']=='fresh-machine'
PY
echo 'PASS completely fresh installation with absent units'
reset_fixture false false
rm "$BOOPS_ROOT/usr/local/bin/boops" "$BOOPS_ROOT/etc/systemd/system/boops.service" "$BOOPS_ROOT/etc/systemd/system/boops.timer" "$BOOPS_ROOT/etc/boops/config.json"
export FAIL_REGISTRATION=1
if bash "$work/installer.sh" fresh-machine > "$work/output" 2>&1;then exit 1;fi
test ! -e "$BOOPS_ROOT/usr/local/bin/boops"
test ! -e "$BOOPS_ROOT/etc/systemd/system/boops.service"
test ! -e "$BOOPS_ROOT/etc/systemd/system/boops.timer"
test ! -e "$BOOPS_ROOT/etc/boops/config.json"
! rg -q 'restoration also reported an error' "$work/output"
python3 - "$FIXTURE_STATE" <<'PY'
import json,sys
s=json.load(open(sys.argv[1]));assert not s['enabled'] and not s['active']
PY
echo 'PASS completely fresh registration failure restores absent installation'
for state in active activating;do
    reset_fixture false false
    python3 - "$FIXTURE_STATE" "$state" <<'PY'
import json,sys
p=sys.argv[1];s=json.load(open(p));s['service_status']=sys.argv[2];s['service_active']=sys.argv[2]=='active';open(p,'w').write(json.dumps(s))
PY
    export FAIL_RELOAD=1
    run_failure;assert_rollback false false
    python3 - "$FIXTURE_STATE" "$FIXTURE_LOG" <<'PY'
import json,sys
s=json.load(open(sys.argv[1]));assert s['service_status']=='active'
lines=open(sys.argv[2]).read().splitlines();start=lines.index('systemctl start boops.service')
for suffix in ('/usr/local/bin/boops','/etc/systemd/system/boops.service','/etc/systemd/system/boops.timer'):
 assert any(v.startswith('mv ') and v.endswith(suffix) for v in lines[:start]),suffix
assert 'systemctl daemon-reload' in lines[:start]
PY
    echo "PASS rollback resumes $state oneshot service with inactive timer"
done
reset_fixture true true;export FAIL_STOP_UNIT=boops.service
run_failure
assert_rollback true true
! rg -q '^curl ' "$FIXTURE_LOG"
echo 'PASS genuine stop failure aborts before download'
echo 'All installer fixtures passed; no real systemd or NIC operation was performed.'
