# Boops Client

Boops Client は、WebUI で登録したマシンの設定を API から取得し、IPv4 ネットワーク設定を同期するクライアントです。Linux ではホスト名も同期します。新規の IP・ゲートウェイ・DNS 設定は IPv4 のみを受け付けます。0.3.0 から Linux amd64・arm64 の署名付き自動更新に対応します。既存の 0.1／0.2 クライアントには更新機能がないため、初回だけインストーラーで移行してください。

## 新規導入と 0.1／0.2 からの移行

対象は systemd を使用する Linux amd64・arm64 です。root 権限と Bash、Python 3.6 以上、OpenSSL 3.0 以上、curl、systemctl、flock、および通常のファイル操作コマンドが必要です。OpenSSL で Ed25519 署名を検証できない環境では導入を中止します。

```bash
curl -fsS --proto '=https' https://file.booyah.dev/BoopsDB-Client/install.sh -o install.sh
sudo bash install.sh
```

新規導入では、先に WebUI でマシンと NIC を作成し、そのマシン ID を指定します。対話入力の代わりに `sudo bash install.sh <machine-id>` でも指定できます。`regist` は `GET /api/machines/:id` で既存マシンと ID の一致を確認してから端末へ ID を保存します。サーバーの NIC 設定は登録操作で変更しません。登録に成功してからタイマーを有効化します。

既存の `/etc/boops/config.json` があれば再登録せず、UUID、未知の設定キー、`machine_state.json` のバイト列を保持します。移行時は存在する timer と service を停止してから署名・サイズ・SHA-256・`version` の起動検査を行い、検証済みバイナリへ置換します。移行前の timer の enabled／active 状態を復元するので、無効化していた端末で同期を勝手に有効化しません。途中で失敗した場合は旧バイナリと unit ファイルを戻し、タイマーの状態も復元します。停止前に実行中だった service は、旧バイナリと unit の復元後に再開します。復元自体に失敗した場合は、表示された一時ディレクトリに復旧用コピーを残します。

配布先は [BoopsDB-Client](https://file.booyah.dev/BoopsDB-Client/) です。[install_0.3.sh](https://file.booyah.dev/BoopsDB-Client/install_0.3.sh) と `install.sh` は同一内容です。旧インストーラーと 0.1／0.2 バイナリは履歴として保持します。

## 同期と更新

```bash
boops version
sudo boops update
sudo boops sync
sudo boops regist <machine-id>
```

`version` は `0.3.0` などのバージョンだけを出力します。開発ビルドの既定値は `dev` です。`sync` は設定を読み込んだ後、API の取得前に更新を確認します。更新できた回の同期は終了し、次のタイマー起動から新版を使います。更新確認に失敗した回も現行クライアントで通常の同期を続けます。ネットワーク設定の検証・適用に失敗した場合は比較用 state を保存せず、次回も再試行します。

Linux のホスト名変更に失敗した場合は警告を表示してネットワーク同期を続け、変更できなかったホスト名を state に記録しません。次の同期で再試行します。Windows では Linux の hostnamectl を呼ばず、既存のネットワーク同期へ進みます。Windows のホスト名変更や再起動は行いません。

タイマーは毎分起動しますが、最新版の取得は既定で 1 時間に 1 回です。通信失敗後も次の定期確認まで待ちます。`boops update` は間隔を待たずに確認します。開発ビルドと Linux amd64・arm64 以外の環境ではバイナリを自動置換しません。

`/etc/boops/config.json` の `auto_update` がない場合は自動更新が有効です。無効にする場合は既存の ID と他のキーを保ち、`"auto_update": false` を追加します。

```json
{"id":"既存のマシンID","auto_update":false}
```

`auto_update` は定期同期の更新確認を制御します。明示的な `boops update` は無効化中でも実行できます。再登録は ID だけを更新し、`auto_update` と未知の JSON キーを保持します。

更新の確認開始時刻は `/etc/boops/update-state.json` の `last_check` に UTC で保存します。UUID の config とネットワークの state は更新処理から書き換えません。`/etc/boops/update.lock` のプロセス間ロックにより、同時に置換する処理を一つに制限します。定期同期がロックを取得できない場合は更新をスキップし、通常同期へ進みます。手動更新ではロック競合をエラーとして表示します。

## 署名と配布

固定の HTTPS 配布先から `latest.json` を取得し、ソースに埋め込んだ Ed25519 公開鍵で署名を検証します。署名は Base64 を decode した payload の生バイトに対するものです。検証後に OS／arch 別のファイル名・サイズ・SHA-256 を読み、新しい版だけを取得します。manifest は 1 MiB／15 秒、バイナリは 64 MiB／120 秒を上限とします。HTTP エラー、署名不一致、ハッシュ不一致、起動検査失敗では現行バイナリを保持します。

公開鍵は [update/public-key.pem](update/public-key.pem) とインストーラーに固定しています。初回リリースの秘密鍵は `/Volumes/DATAHDD1/BoopsDB-release-private/signing-key.pem` に保存し、ディレクトリを 0700、ファイルを 0600 に限定します。秘密鍵はリポジトリと配布先へ含めません。同じ鍵を安全な場所へ別途バックアップしてください。

ローカル生成では、まだ存在しない出力ディレクトリを指定します。release tool が両アーキテクチャを `CGO_ENABLED=0` でビルドし、署名済み manifest、公開鍵、チェックサムとまとめて生成します。

```bash
cd boops-client
go test ./...
bash test/install-test.sh
go run ./cmd/release -version 0.3.0 \
  -key /Volumes/DATAHDD1/BoopsDB-release-private/signing-key.pem \
  -out /Volumes/DATAHDD1/BoopsDB-releases/0.3.0
```

生成後に同じ出力先へ `install_0.3.sh`、同一バイトの `install.sh`、検証記録 `verification-ja.md` を用意し、`python3 scripts/publish.py --dir <release-dir>` で検証します。公開するときだけ `--publish` を付けます。公開処理は両バイナリと bootstrap ファイルを読み戻して検証し、最後に `latest.json` を公開します。同じバージョンの異なる内容は上書きしません。

GitHub Actions の PR と main push は試験・ビルドだけを行います。公開は `boops-client/v0.3.0` のようなタグ、または `release=true` とバージョンを指定した手動 dispatch に限定します。今後 CI で公開するには、管理者がこの公開鍵に対応する秘密鍵の PEM 全文を `BOOPS_CLIENT_SIGNING_KEY` secret に登録する必要があります。この実装から secret を登録することはありません。CI は鍵を権限 0600 の一時ファイルで扱い、処理後に削除します。

鍵を交換するときは、旧鍵で署名した更新に新しい公開鍵を含めて配布します。旧鍵が使えない場合は、初回移行と同じ手動導入で公開鍵を切り替えます。manifest から鍵を受け入れる仕組みはありません。

## 復旧と検証範囲

旧バイナリは `/usr/local/bin/boops.previous` に保持します。手動復旧では timer と service を停止し、コピーを戻して `boops version` を確認した後、元の enabled／active 状態へ戻します。自動更新を止めたい場合は config の `auto_update` を false にしてください。起動後の異常を検知する自動ロールバックは実装していません。

一時ディレクトリ・署名済み fixture・模擬コマンドで、登録、失敗後の state 保持、移行、更新と配布を試験します。実 NIC、実際の Netplan／NetworkManager／ifupdown、systemd、Windows での稼働は、対象端末で確認するまで未検証です。arm64 のクロスビルドも実行確認を意味しません。WebUI／API の本番反映と既存端末の初回移行は、成果物の配布とは別に行ってください。
