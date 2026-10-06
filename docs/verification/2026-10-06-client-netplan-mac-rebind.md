# boops-client 0.3.2 Netplan の古い MAC 照合の修正

## 実機で確認した原因

SSH で Ubuntu 26.04.1 LTS の対象端末を確認した。0.3.1 が導入済みで、Netplan の ens18 に設定された MAC と、現在の ens18 の MAC が異なっていた。古い MAC を持つ別 NIC はローカルに存在しなかった。0.3.1 はこれを別 NIC の定義として拒否していた。

0.3.2 は NIC 名が明示された ethernet 定義に限り、旧 MAC に現在の所有者がいないことと、対象のローカル識別情報を確認してから MAC 照合を更新する。同じ定義が複数ファイルに分かれている場合は全ての MAC 照合を更新する。取得可能な恒久 MAC を優先し、実行時の MAC だけが変更された場合は正しい恒久 MAC の照合を保持する。

対象を特定できない MAC のみのグループ、広い名前パターン、不明な match 条件、実行時 MAC の明示変更、仮想リンク、複数の候補は変更しない。同一 ID の分割ファイルに異なる MAC が指定されている場合も、各ファイルを確認して書き込み前にエラーとする。古い iproute2 では sysfs から NIC と物理デバイスを確認する。適用に失敗した場合は元のファイル内容と権限を戻す。

## 検証

実機から取得した旧 MAC と現在の MAC の組み合わせで、修正前の `Netplan ID ens18 belongs to another NIC` を再現し、修正後の成功を確認した。DHCPv6、set-name、分割定義、generate/apply 失敗時の復元、旧 MAC が別 NIC の現在または恒久 MAC である場合の拒否を試験した。

`go test ./... -count=1` の全244テストと5パッケージが成功し、system/client の race、全パッケージの vet、diff check も成功した。

独立レビューで未解消の Critical／Important は 0 件。署名付き成果物、実機への適用と配布先の読み戻し結果は、完了後に追記する。

実機適用前に設定・バイナリ・比較 state を root 専用のバックアップへ保存した。API の IP と現在の SSH 接続先が異なるため、適用時は元の接続先を復旧する予約を用意し、新しい IP で確認した後に解除する。

[Netplan の MAC 照合と名前変更の仕様](https://netplan.readthedocs.io/en/latest/netplan-yaml/)を参照した。
