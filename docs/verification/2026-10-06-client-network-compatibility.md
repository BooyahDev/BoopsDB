# boops-client 0.3.1 ネットワーク設定の修正

## 原因と修正

0.3.0 は、対象 NIC が `/etc/netplan/01-netcfg.yaml` 以外のファイルで設定されていると競合として拒否していた。Ubuntu のインストーラーが作成する `00-installer-config.yaml` もこの条件に該当した。

0.3.1 は有効な Netplan ファイルを読み、ローカルの NIC 名・MAC と既存定義を照合して、対象の IPv4 設定をそのファイル内で更新する。同じ ID が複数ファイルに分かれている場合も全定義を確認し、旧 IPv4 のアドレス、DNS、デフォルトルートを残さない。MAC 照合、名前変更、DHCPv6、既存 IPv6、対象外の設定を保持する。ベンダーの `/lib/netplan` は同名の `/etc/netplan` ファイルで上書きし、元ファイルは変更しない。新規 NIC を追加する場合も同名の既存設定を引き継ぎ、対象外の NIC や renderer を欠落させない。名前変更前後の NIC 名が異なる場合は、MAC の一致を確認してから更新する。

Debian の ifupdown と Proxmox の ifupdown2 は、メインファイルと読み込み先の設定を扱う。Proxmox のブリッジのメンバー、STP、FD、VLAN などを保持する。IPv4 のない物理ポートも扱う。直接の `iface` 宣言がないポートもブリッジ・ボンド・VLAN の依存関係から設定方式を判定する。既存ブリッジの反映には `ifreload` が必要で、コマンドがない場合は変更前にエラーにする。NetworkManager は既存の接続プロファイルの IPv4 設定を更新する。

OS の名前や版ではなく、既存設定と利用可能なコマンドで設定方式を選ぶ。ループバックだけの `/etc/network/interfaces` が残っていても、Netplan／NetworkManager の使用を妨げない。古い iproute2 で JSON 出力が使えない場合は sysfs のローカル MAC を利用する。

変更する全ファイルの内容と権限を先に取得する。途中の書き込み、設定生成、適用で失敗した場合は変更を戻す。旧 ifupdown の復旧では、元から起動していた NIC だけを再起動し、停止していた NIC を起動しない。変更範囲を特定できない共有 YAML、広い NIC 名のパターン、一時的な `/run/netplan` の対象定義、ifupdown の論理名マッピングは変更前に拒否する。

## 検証記録

提示された `ens18` の Ubuntu 設定を再現テストにした。修正前は利用者と同じ競合エラーになり、修正後は既存ファイルの更新が成功する。`dhcp6: true`、MAC と `set-name` の保持を確認した。

次の検証が成功した。

- `go test ./... -count=1`：220 テスト、5 パッケージ、失敗なし。
- `go test -race ./system ./client -count=1` と `go vet ./...`。
- インストーラーの 20 ケース、Bash 構文、2 つのインストーラーのバイト一致。
- 配布処理の 33 テスト。プロセス起動待ちで期限が切れないよう、通信期限テストの待ち幅だけ調整した。配布処理の実装や本番の制限時間は変更していない。
- Windows amd64 のクロスビルド。

試験中、更新機能の既存テストで、ビルド・race と並行した実行時に 1 秒の試験上限を超えるケースが 1 件あった。実装とそのテストは変更せず、単独実行と、並行ビルドを止めた全体再実行が成功した。

インストーラーの旧 CLI 試験は、この Mac の実 libcrypto を使った署名検証と、旧 CLI の模擬環境を組み合わせている。実 OpenSSL 1.1.1 を導入した OS 上での試験は未実施。

署名付き Linux 成果物の生成と配布先の読み戻しは、公開後に追記する。

実 Debian／Ubuntu／Rocky Linux／Proxmox の NIC、疎通、systemd、再起動後の設定保持は未実施。Mac 上の一時ファイルと模擬コマンドによる試験、クロスビルド、成果物の静的検査を実機検証と区別する。

## 参照した仕様

- [Netplan の設定仕様](https://netplan.readthedocs.io/en/latest/netplan-yaml/)
- [Netplan の設計と設定 ID](https://netplan.io/design)
- [Debian ifupdown の設定仕様](https://manpages.debian.org/bullseye/ifupdown/interfaces.5.en.html)
- [Debian ifupdown2 の再読み込み](https://manpages.debian.org/bullseye/ifupdown2/ifreload.8.en.html)
- [Proxmox のネットワーク設定](https://pve.proxmox.com/wiki/Network_Configuration)
- [OpenSSL 1.1.1 の Ed25519 検証](https://docs.openssl.org/1.1.1/man7/Ed25519/)
