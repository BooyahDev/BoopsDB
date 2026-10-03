# Boops Client 0.3.0 の検証記録

記録日時: 2026-10-04T03:17:02+09:00。バイナリ生成元 commit: `0032b8ca876a5cc4ba31efe6e07f225fc4f0d410`。公開処理の修正 commit: `2df9f4c153fae202846b7798005dd6fcb495244e`。

Linux amd64／arm64 を Go 1.26.5、CGO_ENABLED=0、trimpath、version=0.3.0 の生成コマンドでビルドした。静的 ELF の CPU 種別・共有ライブラリ依存なし・ビルド情報のソース commit と未変更状態、version文字列の存在を確認した。trimpath付きGoビルドはldflagsをメタデータへ記録しないため、メタデータからversion引数の読取りを行ったとは扱わない。macOS amd64 用の別ビルドでは `version` が 0.3.0 を返した。配布する Linux バイナリはこの macOS では実行していない。

生成した manifest は固定公開鍵を使う Go 検証と OpenSSL の Ed25519 検証に成功した。署名対象は Base64 を復号した payload の生バイトで、両バイナリのサイズ・SHA-256 と一致する。配布版公開鍵はソースの固定公開鍵と同一で、秘密鍵は成果物に含めない。

## 成果物

| ファイル | サイズ（byte） | SHA-256 |
| --- | ---: | --- |
| `boops_0.3.0_amd64.binary` | 7168162 | `988a29bba9ab60dca76f6903deb67f094a92a6662c33f913aafa51511e87e7f7` |
| `boops_0.3.0_arm64.binary` | 6619298 | `dfe91f87bdedcbfb315ca4a96b041d7e6d17a248cd1dc65f2e8e33ff49044cca` |
| `latest.json` | 554 | `c0ec62b9bfc1f191c3e8498cb63ac863ee5027b437f29175fa6e91b8a7f2ba6b` |
| `install.sh` | 10200 | `2ce31c3015c23bb4d48bba4656615985224a1210baf2c2b59acc2e3882194a3f` |
| `install_0.3.sh` | 10200 | `2ce31c3015c23bb4d48bba4656615985224a1210baf2c2b59acc2e3882194a3f` |
| `public-key.pem` | 113 | `2255abd88677575c8ec94344979eaa97875c01b58711c6971b7e3da8354e8529` |
| `SHA256SUMS` | 182 | `b99017a2a25a2302d18ac3d5b3cc2cca014edafd3e8013faa8c10a4b3f2bc35a` |

## 実施した検証

| 対象 | 結果 |
| --- | --- |
| API・DB | 隔離 MySQL 8.4.10 で21件成功、skip 0。末尾空白のみ整理後のAPI unit9件も成功 |
| Go | 最終修正後の全5package試験・vet成功。関連client/systemのrace成功 |
| クロスビルド | Linux amd64／arm64、Windows amd64 が成功 |
| 公開処理 | 実配布フォームの契約修正後にPython33件成功。署名、改変、ファイル名付きPOST先、既存成果物保護、manifestの公開順、通信の総期限を確認 |
| インストーラー | 一時ファイル・模擬コマンドによる17件成功。完全新規導入、旧版移行、失敗復元、実行中serviceの復元を確認 |
| WebUI | unit14件、本番build、SSR16要求が成功 |
| Chrome実画面 | PC／390×844、テーマの選択・保存、検索条件の復元、NIC名・IP・DNS・gatewayの編集、追加削除後の順序を確認 |
| 独立レビュー | 個別と全体レビューで未解消のCritical／Important／Minorは0件 |

ゲートウェイはマシン全体で最大1つとし、空値による解除を確認した。大文字UUIDの既存設定を保持して登録・同期でき、NetplanのMAC照合にはローカル実値を使う。既存定義との競合や安全に復元できないifupdown論理名は、変更前に検出する。

公開処理は両バイナリと補助ファイルの読み戻しを検証し、最後にlatest.jsonを公開する。同一バージョンの異なるバイナリは上書きしない。CIのrelease jobも共通concurrency groupで直列化する。公開後の独立した読み戻し結果は別途記録する。

## 初回移行と未実施の確認

旧0.1／0.2には自動更新機能がないため、初回だけ次の手順で移行する。

```bash
curl -fsS --proto '=https' https://file.booyah.dev/BoopsDB-Client/install.sh -o install.sh
sudo bash install.sh
```

移行後の最新版確認は既定で1時間に1回。既存UUID・未知の設定キー・比較stateとtimer状態の保持はfixtureで確認した。新規IP・gateway・DNSの入力はIPv4が対象。

Linux両archの配布バイナリ実行、実NICのNetplan／NetworkManager／ifupdown適用、実systemd、Windows・arm64の稼働、OS配色変更イベント、Docker内起動、GitHub Actions、本番WebUI／APIの反映、既存端末の初回移行はNOT RUN。クロスビルドと模擬試験を実機稼働確認として扱わない。

## 公開後の独立した読み戻し

公開処理は成功し、両バイナリと補助ファイルの読み戻し後に latest.json を公開した。さらに curl で全8ファイルを独立して取得し、ローカル成果物とサイズ・SHA-256を照合した。取得したmanifestは、ソースの固定公開鍵を用いるGoとOpenSSLの双方で署名検証に成功した。両installer aliasと公開鍵のバイト一致も確認した。

読戻し日時（UTC）: `2026-10-03T18:24:21.259248+00:00`。

| ファイル | サイズ（byte） | 読み戻したSHA-256 |
| --- | ---: | --- |
| `boops_0.3.0_amd64.binary` | 7168162 | `988a29bba9ab60dca76f6903deb67f094a92a6662c33f913aafa51511e87e7f7` |
| `boops_0.3.0_arm64.binary` | 6619298 | `dfe91f87bdedcbfb315ca4a96b041d7e6d17a248cd1dc65f2e8e33ff49044cca` |
| `latest.json` | 554 | `c0ec62b9bfc1f191c3e8498cb63ac863ee5027b437f29175fa6e91b8a7f2ba6b` |
| `install.sh` | 10200 | `2ce31c3015c23bb4d48bba4656615985224a1210baf2c2b59acc2e3882194a3f` |
| `install_0.3.sh` | 10200 | `2ce31c3015c23bb4d48bba4656615985224a1210baf2c2b59acc2e3882194a3f` |
| `public-key.pem` | 113 | `2255abd88677575c8ec94344979eaa97875c01b58711c6971b7e3da8354e8529` |
| `SHA256SUMS` | 182 | `b99017a2a25a2302d18ac3d5b3cc2cca014edafd3e8013faa8c10a4b3f2bc35a` |
| `verification-ja.md` | 4440 | `36a74848e4919cb8af7d8b00b2c2f0192845cc2ad9974f8d105eeb20164104ce` |

公開前に記録した既存9ファイル（0.1／0.2の両CPUバイナリ、旧installer2本、service、timer、heartbeat）も再取得し、全件のサイズ・SHA-256が一致した。旧ファイルを削除・上書きしていない。

- [配布ディレクトリ](https://file.booyah.dev/BoopsDB-Client/)
- [最新版manifest](https://file.booyah.dev/BoopsDB-Client/latest.json)
- [公開検証記録](https://file.booyah.dev/BoopsDB-Client/verification-ja.md)
- [移行手順と設定](../../boops-client/README.md)

初回の公開操作は送信先URLがディレクトリになっていたため失敗した。実配布フォームに合わせたファイル名付きPOSTへ修正し、追加のRED→GREENと全33試験・独立レビューを通して再公開した。初回時点で新binaryとmanifestのGETは404で、最終的な成功は上記の読み戻しで確認した。バイナリ生成元0032b8cと公開処理2df9f4cは、変更対象が異なるため分けて記録した。

## 実行時の判断と保管

homeの空き不足で管理worktreeを作成できなかったため、元のクリーンなcheckoutから専用のcodex/console-client-updatesブランチを使用した。キャッシュをDATAHDDへ移し、残存ファイルのSHA-256一致を確認して既存データを保持した。署名鍵はリポジトリと公開成果物の外へ権限0700／0600で保存した。本番のAPI・DB・端末の設定変更は行っていない。

APIはNIC名を厳密一致とし、旧呼出しの空検索を変えずにWebUIの全件表示へall=1を追加した。Netplanの広いmatch、外部定義の競合、ifupdownの異なる論理名など、安全な編集と復元を保証できない構成は変更前に拒否する。署名鍵、構成、UUID、旧バイナリを保持する境界を個別・全体レビューで確認した。

Astraによる0032b8cの全体承認後、2df9f4cのPython限定差分を別の独立reviewerが承認し、元の全体承認を継承した。未解消のCritical／Important／Minorは0件。署名成果物と実公開の検証はコードレビューから分けて記録した。

隔離MySQLは試験終了後に停止済み。検証用WebUIとfixtureはloopbackで保持し、検証用データによるプレビューとして扱う。本番WebUI／APIの反映、既存端末の初回移行、実NIC・systemdなどの実機確認は未実施である。作業ブランチを保持し、GitHubへのpush・merge・PRは行っていない。

ローカルの詳細ログ・読み戻しJSON・画面・レビュー記録は `/Volumes/DATAHDD1/BoopsDB-releases/evidence/` に保管した。配布処理のallowlistにはそれらを含めていない。
