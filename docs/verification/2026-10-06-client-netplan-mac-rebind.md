# boops-client 0.3.2 Netplan の古い MAC 照合の修正

## 実機で確認した原因

SSH で Ubuntu 26.04.1 LTS の対象端末を確認した。0.3.1 が導入済みで、Netplan の ens18 に設定された MAC と、現在の ens18 の MAC が異なっていた。古い MAC を持つ別 NIC はローカルに存在しなかった。0.3.1 はこれを別 NIC の定義として拒否していた。

0.3.2 は NIC 名が明示された ethernet 定義に限り、旧 MAC に現在の所有者がいないことと、対象のローカル識別情報を確認してから MAC 照合を更新する。同じ定義が複数ファイルに分かれている場合は全ての MAC 照合を更新する。取得可能な恒久 MAC を優先し、実行時の MAC だけが変更された場合は正しい恒久 MAC の照合を保持する。

対象を特定できない MAC のみのグループ、広い名前パターン、不明な match 条件、実行時 MAC の明示変更、仮想リンク、複数の候補は変更しない。同一 ID の分割ファイルに異なる MAC が指定されている場合も、各ファイルを確認して書き込み前にエラーとする。古い iproute2 では sysfs から NIC と物理デバイスを確認する。適用に失敗した場合は元のファイル内容と権限を戻す。

## 検証

実機から取得した旧 MAC と現在の MAC の組み合わせで、修正前の `Netplan ID ens18 belongs to another NIC` を再現し、修正後の成功を確認した。DHCPv6、set-name、分割定義、generate/apply 失敗時の復元、旧 MAC が別 NIC の現在または恒久 MAC である場合の拒否を試験した。

`go test ./... -count=1` の全244テストと5パッケージが成功し、system/client の race、全パッケージの vet、diff check も成功した。

独立レビューで未解消の Critical／Important は 0 件。Windows amd64 のクロスビルドも成功した。

## Ubuntu 実機への適用

Ubuntu 26.04.1 LTS、Linux amd64 の実機で適用した。適用前に設定・バイナリ・比較 state を root 専用の `/root/boops-repair-20261006-mac` に保存し、5分後に旧接続先へ戻す systemd の復旧予約を有効にした。API の指定で `10.1.1.3/16` から `10.1.6.4/16` へ切り替わり、新しい IP の SSH と API GET/PUT が成功したため復旧予約を解除した。

確認した結果は次のとおり。

- `boops version` は0.3.2で、実機のバイナリ SHA-256 は署名対象と一致。
- `ens18` の MAC 照合が実機の `bc:24:11:35:84:47` へ更新。DHCPv4 は false、DHCPv6 は true、set-name と IPv6 を保持。
- IPv4 は `10.1.6.4/16`、デフォルト経路は `10.1.0.1` の1本、DNS は `1.1.1.1` と `8.8.8.8`。
- networkd は fallback の `zzzz-dracut-default.network` から `10-netplan-ens18.network` へ切り替わり、routable/online。
- 初回の IP 切り替え時には既存の API 通信がタイムアウトしたが、サービスは終了コード0で完了。以後の手動同期と systemd サービス実行は警告なしで成功。`Result=success`、`ExecMainStatus=0` を確認。
- API の MAC と last_alive/updated_at も更新。MAC が API へ反映された後の再同期では Netplan の SHA-256 と更新時刻が変わらないことを確認。
- `boops.timer` は active に復帰。復旧 timer は inactive。公開後の実機 `boops update` は `not-newer`、version は0.3.2。

Debian・Rocky Linux・Proxmox の設定方式と旧 iproute2 は模擬試験で確認している。今回の実機検証は Ubuntu 26.04.1 LTS/amd64 に限る。ほかの OS/バージョン、arm64 の実行、再起動後の設定保持は未実施。

## 署名付き配布

生成元 commit は `573213f158111f0470b5522df4cf195bdddb3db3`。Go 1.26.5、CGO_ENABLED=0、trimpath で Linux amd64/arm64 を生成し、CPU 種別・静的 ELF・共有ライブラリ依存なし・vcs.modified=false を確認した。

[配布先](https://file.booyah.dev/BoopsDB-Client/)へ両バイナリと補助ファイルを公開し、最後に latest.json を0.3.2へ切り替えた。読み戻した新8ファイルがローカルと完全一致し、旧13ファイルが公開前と同一であることを確認。読み戻した成果物に対する固定公開鍵の Go/OpenSSL Ed25519 署名検証、サイズ、SHA-256、生成元 commit の検証も成功した。

| ファイル | サイズ（byte） | SHA-256 |
| --- | ---: | --- |
| `SHA256SUMS` | 182 | `1706dea5c0b21510f9e7baa46e11ca526be2690a9115c3ee059745cbb6d5bfd9` |
| `boops_0.3.2_amd64.binary` | 7217314 | `8aa2099c4effe3db234bcd1f7ddce96419d7e830381540da98eeb065132e1e54` |
| `boops_0.3.2_arm64.binary` | 6684834 | `56c294eabf10929d47ba0473a21d4423d0f682009d7c26254d89401e128b895c` |
| `install.sh` | 14553 | `de470bf4da87bce7a27e17a75767d07a8755bf4dcfe7cc8dbeb0117a050d4e45` |
| `install_0.3.sh` | 14553 | `de470bf4da87bce7a27e17a75767d07a8755bf4dcfe7cc8dbeb0117a050d4e45` |
| `latest.json` | 554 | `fed857631c1804df1b36172c84330accc1da6d532fc8fc2d6a5c8742a5225429` |
| `public-key.pem` | 113 | `2255abd88677575c8ec94344979eaa97875c01b58711c6971b7e3da8354e8529` |
| `verification-ja.md` | 4319 | `0991643c1ee22473ead43ed37e28fce1c2b6ac2b4629de2dcf115bb664dc72a1` |

ローカルの試験・署名・読み戻し記録は `/Volumes/DATAHDD1/BoopsDB-releases/evidence/0.3.2` に保存した。認証情報は記録に含めていない。

[Netplan の MAC 照合と名前変更の仕様](https://netplan.readthedocs.io/en/latest/netplan-yaml/)を参照した。
