# BoopsDB Console and Client Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** WebUI の表示と編集順を整理し、ゲートウェイを選択できるネットワーク反映と、Linux クライアントの検証付き自動更新・配布を完成させる。

**Architecture:** 既存の Vuetify と API の形を維持し、行 ID の保持と machine 行のロックで表示順と gateway 選択を揃える。ネットワーク反映は全 NIC を検証して方式ごとに一括反映し、更新は API 同期前に独立して確認する。API・画面・ネットワーク・更新処理は担当を分け、main と配布処理は最後に統合する。

**Tech Stack:** Nuxt 3、Vue 3、Vuetify 3、Node.js 22 以上、Express 5、mysql2、Go 1.22.2 以上。追加の Go 依存は Netplan 用の [go.yaml.in/yaml/v3 v3.0.5](https://pkg.go.dev/go.yaml.in/yaml/v3@v3.0.5) のみとする。テストは Node 標準の `node:test` と Go 標準の `testing` を使う。

**Spec:** [承認済み設計書](../specs/2026-10-04-console-client-updates-design.md)。設計書の基準は `df408bbad18ac19b48ca9c815df7c8a81f90e1b4`、承認時の文書コミットは `7c5a903`。計画作成中に確認した登録 API の不整合は、設計書の初回移行節へ補足した。

## Global Constraints

- 対象は `boops-webui`、必要な `boops-server` の API、`boops-client`、クライアントのビルド・配布処理と説明書である。旧 `boops-ui` と `boops-cli` は今回の変更対象に含めない。
- 稼働中の DB、端末、WebUI の本番環境にはこの開発作業から設定変更を実行しない。修正版クライアントの配布先へのアップロードはユーザーの依頼範囲に含む。
- テーマは「システム」「ライト」「ダーク」の 3 通り。選択を Cookie に保存し、SSR は背景付きの `ClientOnly` プレースホルダーを使う。
- マシンの既定 SQL は `created_at IS NULL ASC, created_at ASC, id ASC`。NIC・IP は `id ASC`。明示ソートの同値条件はマシン ID とし、任意のドラッグ並べ替えは追加しない。
- 同じマシンで非空ゲートウェイを持つ NIC を 0 または 1 つとする。空文字、`null`、空白のみ、`0.0.0.0` は空文字に正規化する。今回のネットワーク適用は現行と同じ IPv4 を対象とする。
- ネットワーク適用が成功した場合だけ `machine_state.json` を保存する。UUID を持つ `config.json` はネットワーク変更や更新で書き換えない。
- 自動更新の対象は Linux amd64・arm64。更新確認は既定で 1 時間に 1 回。`auto_update: false` で無効、欠落時は有効とする。API 同期前に確認し、更新失敗では通常同期を続ける。
- `latest.json` は標準 Base64 の `payload` と `signature`。署名対象は decode 後の UTF-8 生バイト、署名は 64 バイトの Ed25519。Go の埋込み公開鍵は 32 バイト、インストーラーは同じ鍵の SubjectPublicKeyInfo PEM。
- manifest は 1 MiB、バイナリは 64 MiB を上限とし、manifest 取得は 15 秒、バイナリ取得は 120 秒で打ち切る。固定 HTTPS 配布先以外への取得・リダイレクトは拒否する。
- 初回版は `0.3.0`、配布先は `https://file.booyah.dev/BoopsDB-Client/`。`boops_0.3.0_amd64.binary` と `boops_0.3.0_arm64.binary` を新規配布し、既存 0.1／0.2 ファイルを上書きしない。
- 初回導入は Python 3、OpenSSL 3.0 以上を必要とする。秘密鍵はリポジトリと配布先に保存しない。旧版コピーは手動復旧用であり、起動後の自動ロールバックは実装しない。
- 実 NIC・systemd・Windows の稼働を試験していない場合は未検証と報告する。クロスビルドと生成結果の試験を実機確認と扱わない。

## Review Focus

- 同値ソート・同名マシン・同一内容の ID なし IP があっても、ページングと再保存で行を取り違えない（Task 1 の integration test）。
- 別 NIC への gateway 保存が同時に来ても最大 1 つを維持し、無効な一括要求が途中保存されない（Task 1 の並行 HTTP test）。
- cloud-init、別 Netplan ファイル、ifupdown の include と default route フックを黙って削除せず、ファイル変更前に競合を返す（Task 2 の fixture test）。
- 更新確認中の時刻巻戻り・破損 state・別プロセス・署名不一致でも、UUID と現行バイナリを保持する（Task 3 の updater test、Task 5 の移行 test）。
- 2 NIC を交互に編集し、OS テーマ変更・再読込・戻る操作を行っても、編集対象、gateway 選択、検索順を取り違えない（Task 4 の fixture API と実画面確認）。

## ファイルと担当の境界

| 担当 | 所有する変更 | 接続先 |
| --- | --- | --- |
| API | `boops-server/app.js`、新規 `httpApp.js`、`networkSettings.js`、`test/`、package scripts と README | Task 4 が GET の IP ID と gateway 専用 PUT の契約を使う |
| ネットワーク | `boops-client/client/config.go`・`types.go` と新規検証コード、`system/`、`go.mod`・`go.sum` | Task 5 が正規化と適用結果を main へ接続する |
| 更新 | 新規 `boops-client/update/`、`cmd/release/`、署名公開鍵ファイル、リリース用テスト | Task 5 が `update.Check` を main と導入手順へ接続する |
| WebUI | `boops-webui/` のみ | Task 1 の API を fixture と実画面で確認する |
| 統合・配布 | `boops-client/main.go`、`main_test.go`、インストーラー、service／timer、README、CI、配布成果物 | 全担当の境界を接続し、公開する |

担当は他の変更を取り消さず、所有範囲外を変更する必要があれば統合担当へ連絡する。main、インストーラー、CI は並行担当に編集させない。Task 1・2・3 は境界を確定してから並行可能、Task 4 は Task 1 の API 契約を使い、Task 5・6 は統合後に行う。

## 実行環境とテストの準備

Node は `/Users/nakamura/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node`、pnpm は同 runtime の `bin/fallback/pnpm` を利用できる。実装開始後に WebUI の lockfile に従って依存を導入する。Docker の現在の context は `desktop-linux` だが、計画作成時の info 取得は完了しなかった。DB 試験前に daemon への接続を短い timeout で再確認する。

API integration test は専用の一時 MySQL 8.4 と `boops_test` DB を使い、production の `models/db.js` や `.env` を import しない。接続先は loopback のポートに限定する。既存 compose を起動せず、テスト専用コンテナーと volume を作る。Docker が利用できない場合は実装・単体試験を先に行い、実 DB 試験を未実施として残す。DB のトランザクション・ロックの保証を fake の結果だけで完了扱いにしない。

---

### Task 1: 順序・行 ID・単一 gateway の API 契約

**Files:** Create `boops-server/httpApp.js`, `networkSettings.js`, `test/networkSettings.test.js`, `test/api.integration.test.js`, `test/start-db.sh`; Modify `boops-server/app.js`, `package.json`, `README.md`。

**Interfaces:**
- `createApp(database) -> Express.Application`。database は mysql2 の `query` と `getConnection` を持つ。listener と production db import は従来の `app.js` に残す。
- `normalizeGateway(value) -> string`、`validateInterfacePayloads(interfaces) -> normalized NIC[]`、`withMachineTransaction(database, machineId, callback) -> Promise<result>`、`reconcileInterfaces(connection, machineId, normalized NIC[]) -> Promise<void>` を `networkSettings.js` に置く。
- GET は現在の `interfaces: []` を維持し、`ips` の各行へ `id` を追加する。PUT は従来の名前キー object と各 value の任意 NIC ID、各 IP の任意 ID を受け付ける。gateway 専用 PUT の URL と成功 response は維持する。

- [x] **Step 1: 失敗する正規化・ID 対応・HTTP 試験を書く。** 最低限の assertion は次とし、同名マシンの NULL 日時、ID なし重複 IP、未知 ID、一括 PUT rollback、旧 request／response を追加する。

```js
assert.equal(normalizeGateway(null), '');
assert.equal(normalizeGateway(' 0.0.0.0 '), '');
assert.throws(() => validateInterfacePayloads({eth0: {ips: [ip], gateway: '10.0.0.1'}, eth1: {ips: [ip2], gateway: '10.0.1.1'}}));
assert.deepEqual(after.interfaces.map(i => i.id), before.interfaces.map(i => i.id));
assert.deepEqual(after.interfaces[0].ips.map(ip => ip.id), before.interfaces[0].ips.map(ip => ip.id));
assert.equal(clearAgain.status, 200);
assert.equal(final.interfaces.filter(i => i.gateway !== '').length, 1);
```

- [x] **Step 2: RED を確認する。** `node --test test/networkSettings.test.js` と専用 DB 接続を指定した `node --test test/api.integration.test.js`。未実装 export または現行の ID 変更・同値 404・複数 gateway で失敗することを確認する。接続不可だけの失敗を RED と扱わない。
- [x] **Step 3: factory と共有の検証・トランザクション処理を実装する。** 既存 middleware／無関係な routes の挙動を保持して `httpApp.js` に移す。名前を使う更新は 0 行なら 404、複数行なら 409。全 payload の検証後に machine 行、子行の順でロックする。新規 machine、NIC 作成、全 machine PUT、gateway／IP／DNS／name／MAC 更新、NIC／machine 削除で同じルールを使う。存在しない登録 POST の alias は作らない。
- [x] **Step 4: 順序と差分保存を実装する。** 全 GET と検索に設計の ORDER BY を適用する。既存 NIC／IP ID を維持して更新し、除外行を IP、NIC の順で削除、新規行を末尾へ追加する。legacy IP は内容と出現順で一度ずつ対応付ける。すべての validation・SQL 失敗で rollback／connection release を確認する。
- [x] **Step 5: GREEN と並行保存を確認する。** `node --test test/*.test.js`。並行して eth0 と eth1 を選択する HTTP request の後に非空 gateway が 1 つであること、無効 PUT 後に machine と子の全値・ID が不変であることを実 DB で確認する。README に空値・単一 gateway・ID・順序・400／409 を記載する。
- [x] **Step 6: 検証済み API の変更をコミットする。** message: `fix: 編集順と単一ゲートウェイのAPI契約を安定化`。

### Task 2: 全 NIC の検証・一括反映・設定保持

**Files:** Modify `boops-client/client/config.go`, `client/types.go`, `system/network.go`, `system/info.go`, `system/utils.go`, `go.mod`; Create `client/network_config.go`, `client/network_config_test.go`, `system/network_ops.go`, `system/network_test.go`, `system/netplan.go`, `system/netplan_test.go`, `system/interfaces.go`, `system/interfaces_test.go`, `system/networkmanager.go`, `system/networkmanager_test.go`, `system/windows.go`, `system/windows_test.go`, `go.sum`。

**Interfaces:**
- `client.NormalizeInterfaces([]client.InterfaceInfo) ([]client.InterfaceInfo, error)`。NIC 名、IPv4、連続 subnet mask、DNS、MAC と gateway 数を検証する。入力を破壊せず gateway を正規化する。
- `client.InterfacesEqual(a, b []InterfaceInfo) bool` は NIC 名と内容を比較し、IP 順序には依存しない。
- `system.ApplyNetworkSettingsWithOps(ifaces []client.InterfaceInfo, ops Ops) error`、`system.RealOps() Ops`。既存 `ApplyNetworkSettings(interface{}) error` は互換 wrapper とする。
- `Ops` は `OS() string`, `Run(name string, args ...string) ([]byte,error)`, `ReadFile(path string) ([]byte,error)`, `WriteFile(path string, data []byte, mode fs.FileMode) error`, `Rename(oldPath,newPath string) error`, `Remove(path string) error`, `Glob(pattern string) ([]string,error)`, `Exists(path string) (bool,error)`, `LookPath(name string) (string,error)` を持つ。本番では argv で実行し、試験では全操作を一時ファイルと記録へ差し替える。

- [x] **Step 1: 失敗する検証・比較試験を書く。** `TestNormalizeInterfacesSingleGateway`、`TestNormalizeInterfacesRejectsBeforeApply`、`TestInterfacesEqualUsesNameAndAllFields`。空 gateway、2 gateway、重複／空 NIC 名、非連続 mask、IP 順序変更、DNS・MAC・dns_register の差分を固定する。

```go
got, err := client.NormalizeInterfaces(twoNICsWithOneGateway)
if err != nil || got[1].Gateway != "" { t.Fatalf("unexpected normalization: %v %v", got, err) }
if _, err := client.NormalizeInterfaces(twoGateways); err == nil { t.Fatal("multiple gateways accepted") }
if !client.InterfacesEqual(original, sameIPsInReverseOrder) { t.Fatal("display order caused reapply") }
```

- [x] **Step 2: RED を確認する。** `go test ./client ./system`。未実装関数、現行の map 最終 NIC だけの Netplan、空 gateway 残存、first-IP キー比較を検出する。
- [x] **Step 3: 正規化・比較・NIC 収集を実装する。** `net/netip` と `net.IPMask.Size()` を使い、非連続 mask を拒否する。NIC 収集で `Name` と実際の `MacAddress` を分ける。CIDR・DNS を方式別に別解釈せず、正規化済み値を利用する。
- [x] **Step 4: Ops と一括バックエンドを実装する。** YAML Node を使い `01-netcfg.yaml` の対象外設定とコメントを保持する。設定の探索・NIC 存在確認・生成をすべて書込み前に行う。Netplan の別ファイルや ifupdown include／route フックは競合を返す。nmcli は active connection を NIC に対応付けて default route の解除を明示し、通常の静的経路を保持する。Windows は gateway なしを明示し、全コマンドの結果を確認する。
- [x] **Step 5: 失敗時復元の fixture 試験を追加して GREEN を確認する。** `TestNetplanKeepsTwoNICsAndUnmanagedNodes`、`TestForeignNetplanAndInterfacesHooksPreventWrites`、`TestNmcliClearsGatewayAndKeepsStaticRoutes`、`TestApplyFailureRestoresConfig`。無効要求・外部競合では書込みと apply が 0 回、正常 Netplan は apply が 1 回、適用失敗では元のバイト列・mode が復元されることを確認する。`go test ./client ./system` を実行する。
- [x] **Step 6: ネットワーク処理をコミットする。** message: `fix: 複数NICのゲートウェイ解除と一括反映を修正`。main の変更は統合担当へ渡す。

### Task 3: 署名付き更新処理と成果物の生成

**Files:** Create `boops-client/update/manifest.go`, `update/manifest_test.go`, `update/updater.go`, `update/updater_test.go`, `update/lock_unix.go`, `update/lock_unix_test.go`, `update/lock_other.go`, `update/public_key.go`, `update/public_key_test.go`, `update/public-key.pem`, `boops-client/cmd/release/main.go`, `cmd/release/main_test.go`。

**Interfaces:**
- `update.Envelope { Payload string; Signature string }`。signed payload は `{schema:1, version:"0.3.0", artifacts:{"linux/amd64":{path,size,sha256},"linux/arm64":{path,size,sha256}}}` とする。
- `update.VerifyManifest(data []byte, publicKey ed25519.PublicKey) (Manifest,error)`、`update.CompareVersions(a,b string) (int,error)`、`update.TrustedPublicKey() (ed25519.PublicKey,error)`。
- `update.Check(ctx context.Context, options Options) (Result,error)`。Options の `CurrentVersion`, `BaseURL`, `ExecutablePath`, `StateDir`, `GOOS`, `GOARCH` は string、`PublicKey` は ed25519.PublicKey、`Force` は bool、`HTTPClient` は `*http.Client`、`Now` は `func() time.Time`、`Probe` は `func(context.Context,string,string) error`。Result は `Updated bool`, `Version string`, `SkippedReason string`。本番の GOOS／GOARCH は runtime の値を使い、試験は Linux target、HTTP／時計／実行検査を fixture へ差し替える。
- release tool の command は `go run ./cmd/release -version 0.3.0 -key <private-key-pem> -out <output-dir>`。PKCS#8 の Ed25519 秘密鍵を読む。manifest の署名と検証、サイズ・hash の生成を共通コードで行う。

- [x] **Step 1: 失敗する manifest・更新試験を書く。** `TestVerifyManifestSignsRawDecodedPayload`、`TestCheckKeepsCurrentOnVerificationFailure`、`TestCheckCadenceAndLock`、`TestCheckPreservesRegistrationState`。新版・同版・旧版、バージョン `0.3.9` と `0.3.10`、payload 改変、再シリアライズ、別 OS／arch、path traversal、外部 redirect、state 破損／未来、同時実行を固定する。

```go
result, err := update.Check(ctx, optsWithSignedRelease)
if err != nil || !result.Updated { t.Fatalf("update failed: %v %v", result, err) }
if !bytes.Equal(configAfter, configBefore) { t.Fatal("registration config changed") }
if !bytes.Equal(stateAfter, stateBefore) { t.Fatal("machine state changed") }
```

- [x] **Step 2: RED を確認する。** `go test ./update ./cmd/release`。fixture は `httptest` のローカル HTTPS と一時ディレクトリだけを使う。配布中の binary を実行しない。
- [x] **Step 3: 署名・HTTP・バージョン・cadence とロックを実装する。** Go 標準ライブラリで署名を検証し、署名後にだけ payload の値を使う。時刻・サイズ・timeout・redirect は Global Constraints 通りにする。ロック後に state を読み、定期実行はロック競合をスキップ、Force は競合エラーとする。`lock_unix.go` は Linux／Darwin の flock を使い、Mac 上でも一時ディレクトリの fixture を試験できるようにする。実際の非 Linux CLI は GOOS 判定で自動置換を無効にし、Windows の stub を含めてクロスビルドを維持する。
- [x] **Step 4: 検証・旧版保持・原子的置換を実装する。** 同じディレクトリの temp、size／hash、mode、fsync、`version` probe、旧版コピー、rename の順で処理する。旧版コピーや state 保存の失敗を注入する試験でも現行版を破損させない。置換成功後の state 保存失敗は「更新済み」を失敗前保持と混同せず報告する。
- [x] **Step 5: release tool と鍵の固定方法を実装して GREEN を確認する。** リリース用秘密鍵は `/Volumes/DATAHDD1/BoopsDB-release-private/signing-key.pem` へ directory 0700／file 0600 で生成し、既存鍵があれば再生成しない。公開鍵を `public-key.pem` と Go の埋込み鍵へ揃え、release tool でも秘密鍵に対応する公開鍵の一致を確認する。テストは fixture 鍵を用い、本番秘密鍵を出力しない。OpenSSL で Go 生成 payload の署名検証を確認する。`go test ./update ./cmd/release`。実行時に home の空き不足が判明したため、鍵の保存先を DATAHDD1 へ変更した。
- [x] **Step 6: 更新処理をコミットする。** message: `feat: 署名付きクライアント自動更新を追加`。秘密鍵、release binary、一時 state は stage しない。

### Task 4: Console 風 WebUI・テーマ・NIC 編集

**Files:** Modify `boops-webui/layouts/default.vue`, `plugins/vuetify.ts`, `components/AppBar.vue`, `components/SideMenu.vue`, `components/machine/MachineBasicInfo.vue`, `components/machine/MachineActions.vue`, `components/machine/InterfaceCard.vue`, `components/machine/InterfaceAddForm.vue`, `components/machine/InterfaceEditModal.vue`, `pages/index.vue`, `pages/machines/index.vue`, `pages/machines/register.vue`, `pages/machines/[id].vue`, `composables/useMachineApi.js`, `composables/useInterfaceApi.js`, `apiConfig.js`, `nuxt.config.ts`, `Dockerfile`, `package.json`; Create `utils/themePreference.js`, `utils/interfaceRows.js`, `test/uiState.test.js`, `test/fixture-api.mjs`。

**Interfaces:**
- Task 1 の GET の NIC／IP ID、gateway 専用 PUT の選択時の他 NIC 解除を使う。成功後は `getMachine(machineId)` で全カードを更新する。
- `resolveTheme(preference, prefersDark) -> 'light'|'dark'`、`createIpEditRows(ips, nextId) -> editable rows`、`toIpPayload(rows) -> IP[]`。保存済み row の `id` を保持し、新規 row の画面 ID は API へ送らない。
- `useApiBaseUrl() -> string` を setup／composable 初期化時に呼ぶ。Nuxt runtimeConfig の `public.apiBaseUrl` を使い、既定は現行 URL、試験は `NUXT_PUBLIC_API_BASE_URL` で loopback fixture に限定する。

- [x] **Step 1: 失敗する状態試験を書く。** システムテーマ解決、無効 Cookie、新規 IP の固定画面 ID、保存済み IP ID の保持を Node の test で固定する。

```js
assert.equal(resolveTheme('system', true), 'dark');
assert.equal(resolveTheme('light', true), 'light');
assert.deepEqual(toIpPayload(createIpEditRows([{id: 42, ip_address: '10.0.0.2', subnet_mask: '255.255.255.0'}], nextId))[0].id, 42);
```

- [x] **Step 2: RED を確認する。** `node --test test/uiState.test.js`。未実装 helper または ID 欠落で失敗することを確認する。lockfile に従って既存依存を導入し、追加の UI framework は導入しない。
- [x] **Step 3: shell・テーマ・ナビを実装する。** theme Cookie `boops-theme`、システム監視の登録／解除、ClientOnly fallback、`v-main`、ヘッダーと PC／mobile drawer を実装する。ホーム・検索・登録・詳細の余白と操作バーを揃え、固定色を theme token へ移す。Docker の Node を 22 に揃える。
- [x] **Step 4: NIC の境界と編集を実装する。** 各 NIC の識別情報、IP、gateway／DNS を独立カード内に配置する。gateway の選択と解除、既存複数値の通知、再取得後の旧 NIC 解除表示を揃える。IP の最後の行の削除を無効にし、ID を保持して送る。表の行は UUID、明示ソートと query の復元はサーバーと同じ条件にする。
- [x] **Step 5: GREEN と実画面を確認する。** `node --test test/uiState.test.js` と `pnpm build`。fixture API には 2 NIC、複数 IP、同名マシン、複数 gateway のケースを持たせる。ブラウザーツールで両テーマ、システム変更、再読込、戻る、PC／狭い画面、キーボード操作、2 NIC の交互編集を確認し、console の hydration error と誤った NIC の変更がないことを記録する。本番 API へ書込み要求を送らない。
- [x] **Step 6: WebUI をコミットする。** message: `feat: Console風WebUIと安定したNIC編集を追加`。

### Task 5: main・登録・初回移行・リリース CI の統合

**Files:** Modify `boops-client/main.go`, `client/config.go`, `boops.service`, `boops.timer`, repository-root `.github/workflows/build.yml`, root `README.md`; Create `boops-client/main_test.go`, `client/config_test.go`, `install.sh`, `install_0.3.sh`, `test/install-test.sh`, client `README.md`。

**Interfaces:**
- Task 2 の NormalizeInterfaces、ApplyNetworkSettingsWithOps、Task 3 の Check を接続する。version の変数 `main.version` は開発時 `dev`、リリースは ldflags で `0.3.0` とする。
- `syncNetworkState(machine, previous, apply, save) error` は `apply func([]client.InterfaceInfo) error`、`save func(*client.MachineState) error` を受ける小さな同期境界とする。
- `registerMachine(machineID, fetchMachine, saveID) error` は `fetchMachine func(string) (client.Machine,error)`、`saveID func(string) error` を受け、既存マシン確認後だけ ID を保存する。Config の `AutoUpdate *bool` と `AutoUpdateEnabled() bool` を追加し、ID 保存は未知の JSON キーも保持する。

- [x] **Step 1: 失敗する統合・移行試験を書く。** `TestSyncApplyFailureDoesNotSaveState`、`TestRegisterBindsExistingMachineWithoutRemoteWrites`、`TestAutoUpdateDefaultsAndUnknownKeysSurviveRegistration`。shell の fixture は systemctl／curl／openssl と root path を一時ディレクトリへ差し替え、既存 UUID と state のバイト列、timer の enabled／active、途中失敗の復元順を assertion する。

```go
if saves != 0 { t.Fatal("state saved after failed network apply") }
if requests != "GET /api/machines/"+machineID { t.Fatal("registration changed remote settings") }
if !bytes.Equal(configAfterMigration, configBeforeMigration) { t.Fatal("existing UUID/config changed") }
```

- [x] **Step 2: RED を確認する。** `go test . ./client` と `bash test/install-test.sh`。現行の失敗後保存、404 の登録成功表示、installer の早期 timer 起動と直接上書きを検出する。
- [x] **Step 3: main と config を接続する。** `sync` は config 読込み、更新確認、新版置換時は終了、API 同期の順にする。更新失敗は通常同期へ進む。ネットワーク検証・適用が成功した場合だけ状態を保存する。`version` と `update` を追加する。`regist` は GET の成功・UUID 一致を確認してからローカルに保存し、存在しない POST を送らない。
- [x] **Step 4: インストーラーを実装する。** 一つの実体を `install_0.3.sh` とし、release 生成時に同一バイトの `install.sh` を用意する。必須 command を停止前に確認し、既存 timer 状態記録、timer／service 停止、署名検証、hash／version 検証、一時ファイルから置換、状態復元の順にする。service／timer はスクリプトに埋め込む。既存 config があれば regist は呼ばない。失敗時は旧 binary・service ファイル・UUID を保持／復元する。
- [x] **Step 5: CI と説明を揃えて GREEN を確認する。** PR と main は test／build のみ。`boops-client/v*` タグまたは明示的 release dispatch で両 arch を同じ job 集合として生成し、署名鍵を `BOOPS_CLIENT_SIGNING_KEY` secret から取得する公開 job だけで配布する。鍵とログの内容を出力せず、公開済み同 version は異なる hash で上書きしない。GitHub secret の登録は自動で行わず、今後の CI 利用条件として説明する。README に一度だけの移行、`auto_update`、version／update、鍵保管と復旧、実機未検証を記載する。`go test ./...`、`bash -n install_0.3.sh`、`bash test/install-test.sh`、`git diff --check`。
- [x] **Step 6: 統合をコミットする。** message: `feat: 登録と初回移行を更新処理へ統合`。

### Task 6: 全体検証・0.3.0 配布・読み戻し

**Files:** Create `boops-client/scripts/publish.py`, `test/publish_test.py`、ローカル成果物ディレクトリ `/Users/nakamura/Documents/Codex/BoopsDB/releases/0.3.0/`。必要な説明だけ README に追記する。生成 binary と秘密鍵をリポジトリへ含めない。

**Interfaces:** publish script は `python3 scripts/publish.py --dir <release-dir> [--publish]`。既定は検証のみ。明示 publish では固定配布先へ multipart POST の `blob` field を使い、各ファイルの GET 読み戻しを行う。Task 3 の release tool が生成した `latest.json` を両 binary の再取得検証後に最後に公開する。

- [ ] **Step 1: 失敗する公開順・読み戻し試験を書く。** `TestManifestIsLastAfterBothBinaryReadbacks`、`TestReadbackMismatchPreventsManifestPublish`、`TestExistingDifferentVersionArtifactIsNotOverwritten`。HTTP fixture に保存順を記録し、失敗しても latest を公開しないことを assertion する。

```python
self.assertEqual(uploads[-1], 'latest.json')
self.assertNotIn('latest.json', uploads_after_readback_mismatch)
self.assertEqual(existing_release_bytes, original_release_bytes)
```

- [ ] **Step 2: RED を確認して publish script を実装する。** `python3 -m unittest discover -s test -p 'publish_test.py'`。script はローカル signature と hash を確認してから送信し、HTTP status・読み戻しの size／hash を必須とする。アップロード先の既存 0.1／0.2 と無関係なファイルを削除しない。
- [ ] **Step 3: 全体レビューと検証を実行する。** API unit／実 DB integration、Go 全 test、WebUI build／fixture 実画面、installer test を実施する。担当外の fresh reviewer に API／ネットワークと署名境界を含む branch 全体を確認させ、重要な指摘を修正して該当検証を再実行する。実 DB の未実施は明記し、保証済みとは報告しない。
- [ ] **Step 4: 両 Linux artifact を生成する。** `go run ./cmd/release -version 0.3.0 -key /Volumes/DATAHDD1/BoopsDB-release-private/signing-key.pem -out /Volumes/DATAHDD1/BoopsDB-releases/0.3.0`。release tool 自体が両 arch を `CGO_ENABLED=0`、`-trimpath`、`-s -w -X main.version=0.3.0` で生成するため、出力先は未作成のディレクトリとし、手動の先行ビルドは行わない。version の埋込み設定・CPU・静的 ELF・hash・signature を確認し、インストーラーと検証文書を揃える。Linux バイナリの実行確認は、この macOS 上のクロスビルドとメタデータ検査から保証しない。
- [ ] **Step 5: 指定先へ公開して読み戻す。** `python3 scripts/publish.py --dir <dir> --publish`。binary、installer、公開鍵、検証文書を配布し、最後の latest.json を GET して Go／OpenSSL で署名検証する。公開 version・両 arch の size／SHA-256 とローカル値が一致したことを記録する。通常 release と bootstrap の両リンクを確認する。
- [ ] **Step 6: 文書・検証済み処理をコミットし、結果を報告する。** message: `release: boops-client 0.3.0の配布手順と検証を追加`。実装、WebUI／API 本番反映、既存端末の初回移行、実機未検証を分けて報告する。GitHub への push／PR 作成は今回の配布完了の前提にしない。

## 計画のセルフレビュー

設計書の各節を Task 1〜6 に対応付けた。WebUI と gateway の保存契約は Task 1／4、設定保持と再試行は Task 2／5、署名と初回移行・公開は Task 3／5／6 が担当する。Review Focus の 5 条件には担当 task の試験を置いた。main と config の同時編集は Task 2 完了後の Task 5 に限定し、秘密鍵と成果物の配置をリポジトリ外に固定した。

API の新フィールドは ID の追加に限定し、登録は既存 GET を使うため新しい登録 API は追加しない。実 DB・実画面・公開 readback を unit test だけで代用しない。プロダクトへの実装とアップロードは、計画レビューと実行方法の選択後に開始する。

## 実行方法の選択

サブエージェントによる担当別実装と独立レビューを推奨する。API、ネットワーク反映、署名付き更新の誤りは端末の接続や実行ファイルに影響するため、境界ごとにレビューを入れる効果がある。別案はこのエージェントが全 task を順に実装し、最後に独立レビューを 1 回行う方法で、実装中の担当別レビューを減らせる。

## 実行時の検証範囲

Tasks 1–5 は実装・単体／fixture 試験・個別レビューを実施した。API は Docker の代わりに loopback の隔離 MySQL 8.4.10 を使い、実 DB の 21 試験（skip 0）を確認した。WebUI は production build、SSR 16 要求、Chrome の PC／390×844、テーマの選択と保存、編集後の順序と検索条件を確認した。OS 自体の配色変更イベントは未実施である。ネットワーク処理は模擬コマンドと一時ファイルで確認し、実 NIC・systemd・Windows・arm64 の稼働確認を完了扱いにしていない。最終レビュー後の追加修正、生成、公開と読み戻しは Task 6 の記録に残す。
