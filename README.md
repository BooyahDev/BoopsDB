# BoopsDB

BoopsDBは、マシン情報とそのネットワークインターフェースを管理するためのWebアプリケーションです。Node.jsとExpressを使ったサーバー（boops-server）とNuxt.jsで構築されたフロントエンド（boops-ui）から成ります。

## プロジェクト構造

```
BoopsDB/
├── boops-server/      # サーバーサイドのコード
│   ├── .env           # 環境変数
│   ├── app.js         # エントリーポイント
│   ├── package.json   # パッケージ管理
│   ├── models/        # データベースモデル
│   └── sql/           # SQLスキーマとマイグレーション
├── boops-ui/          # フロントエンドのコード（Nuxt.js）
└── README.md          # このREADMEファイル
```

## データベース設定

プロジェクトではMySQLデータベースが使用されています。データベースを初期化するためには、以下のコマンドを実行してください：

```bash
mysql --user root --port 30017 --host 10.0.25.3 --password --execute="source ./schema.sql"
```

## サーバー（boops-server）

### 依存関係のインストール

サーバーディレクトリに移動し、必要なパッケージをインストールします：

```bash
cd boops-server
npm install body-parser express cors mysql2 dotenv uuid
```

### 開発サーバーの起動

```bash
node app.js
```

これでサーバーがhttp://localhost:3001で起動します。

### APIエンドポイント

- `/api/machines` - 全てのマシンを取得する（GET）
- `/api/machines` - 新しいマシンを追加する（POST）
- `/api/machines/:id` - 特定のマシンを更新/削除（PUT/DELETE）

## フロントエンド（boops-ui）

### 依存関係のインストール

フロントエンドディレクトリに移動し、必要なパッケージをインストールします：

```bash
cd boops-ui
npm install
```

### 開発サーバーの起動

```bash
npm run dev
```

これでアプリケーションがhttp://localhost:3000で起動します。

## データベーススキーマ

### machinesテーブル

- `id`: 自動生成されるプライマリキー
- `hostname`: マシンのホスト名
- `model_info`: モデル情報
- `usage_desc`: 使用目的
- `memo`: メモ
- `last_alive`: 最終アクティブ日時
- `created_at`: 作成日時
- `updated_at`: 更新日時

### interfacesテーブル

- `id`: 自動生成されるプライマリキー
- `machine_id`: 所属するマシンのID（外部キー）
- `name`: インターフェース名
- `gateway`: ゲートウェイ
- `dns_servers`: DNSサーバーのリスト（カンマ区切り文字列形式で保存。APIでは配列として取得・送信）
- `mac_address`: MACアドレス

### interface_ipsテーブル

- `id`: 自動生成されるプライマリキー
- `interface_id`: 所属するインターフェースのID（外部キー）
- `ip_address`: IPアドレス
- `subnet_mask`: サブネットマスク

## API使用例

以下にAPIエンドポイントを使うためのcURLコマンドの例を示します。

### 指定されたマシンのlast_aliveを更新する（PUT）

```bash
# このコマンドは指定されたIDのマシンのlast_aliveフィールドを現在の日時に更新します。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-last-alive
```

### 全てのマシンを取得する（GET）

```bash
# このコマンドはローカルで実行されているAPIサーバーから全てのマシン情報を取得します。
curl http://localhost:3001/api/machines
```

### 新しいマシンを追加する（POST）

```bash
# このコマンドは新しいマシンとそのネットワークインターフェースをAPIサーバーに追加します。
curl -X POST http://localhost:3001/api/machines \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "example-host",
    "model_info": "Example Model",
    "usage_desc": "Example usage description",
    "memo": "This is an example machine for testing",
    "last_alive": "2023-01-01 00:00:00",
    "cpu_info": "",
    "cpu_arch": "x86_64",
    "memory_size": "",
    "disk_info": "",
    "interfaces": {
      "eth0": {
        "ips": [
          { "ip_address": "192.168.1.1", "subnet_mask": "255.255.255.0" },
          { "ip_address": "10.0.0.1", "subnet_mask": "255.0.0.0" }
        ],
        "gateway": "192.168.1.254",
        "dns_servers": ["8.8.8.8", "8.8.4.4"],
        "mac_address": "00:11:22:33:44:55"
      },
      "lo": {
        "ips": [
          { "ip_address": "127.0.0.1", "subnet_mask": "255.0.0.0" }
        ],
        "gateway": "",
        "dns_servers": []
      }
    }
  }'
```

### マシンを更新する（PUT）

```bash
# このコマンドは指定されたIDのマシン情報を更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048 \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "updated-example",
    "model_info": "Updated Model",
    "usage_desc": "Updated usage description",
    "memo": "This is an updated example machine",
    "cpu_info": "",
    "cpu_arch": "x86_64",  # Added CPU architecture
    "memory_size": "",
    "disk_info": "",
    "interfaces": {
      "eth0": {
        "ips": [
          { "ip_address": "192.168.1.1", "subnet_mask": "255.255.255.0" },
          { "ip_address": "10.0.0.1", "subnet_mask": "255.0.0.0" }
        ],
        "gateway": "192.168.1.254",
        "dns_servers": ["8.8.8.8", "8.8.4.4"],
        "mac_address": "00:11:22:33:44:55"
      },
      "lo": {
        "ips": [
          { "ip_address": "127.0.0.1", "subnet_mask": "255.0.0.0" }
        ],
        "gateway": "",
        "dns_servers": []
      }
    }
  }'
```

### マシンを削除する（DELETE）

```bash
# このコマンドは指定されたUUIDのマシンとそのインターフェースをAPIサーバーから削除します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X DELETE http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048
```

### 新しいインターフェースをマシンに追加する（POST）

```bash
# このコマンドは指定されたUUIDのマシンに新しいネットワークインターフェースを追加します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X POST http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/interfaces \
  -H "Content-Type: application/json" \
  -d '{
    "name": "eth1",
    "ips": [
      { "ip_address": "192.168.2.1", "subnet_mask": "255.255.255.0" },
      { "ip_address": "172.16.0.1", "subnet_mask": "255.240.0.0" }
    ],
    "gateway": "192.168.2.254",
    "dns_servers": ["1.1.1.1"],
    "mac_address": "aa:bb:cc:dd:ee:ff"
  }'
```

### マシンからインターフェースを削除する（DELETE）

```bash
# このコマンドは指定されたUUIDのマシンから特定のネットワークインターフェースを削除します。例としてマシンUUID=70ae9891-fc07-45b9-8364-3ab159ee2048、インターフェース名="eth1"を使用しています。
curl -X DELETE http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/interfaces/eth1
```

### 仮想マシンの親マシンIDを更新する（PUT）

```bash
# このコマンドは指定されたUUIDの仮想マシンの親マシンIDを更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-parent-id \
  -H "Content-Type: application/json" \
  -d '{
    "parent_machine_id": "e7e3f8a7-bfde-45c7-b9f6-aaa9b97b2c19"
  }'
```

### 仮想マシンのフラグと親IDを更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンの仮想マシンフラグと親マシンIDを更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-vm-status \
  -H "Content-Type: application/json" \
  -d '{
    "is_virtual": true,
    "parent_machine_id": "e7e3f8a7-bfde-45c7-b9f6-aaa9b97b2c19"
  }'
```

### マシンを検索する（GET）

```bash
# このコマンドは指定されたクエリに基づいてマシンを検索します。
curl http://localhost:3001/api/machines/search?q=example-host
```

### UUIDで特定のマシンを取得する（GET）

```bash
# このコマンドは指定されたUUIDのマシン情報を取得します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048
```

### マシンの目的を更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンの使用目的を更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-purpose \
  -H "Content-Type: application/json" \
  -d '{
    "purpose": "New purpose for this machine"
  }'
```

### マシンのホスト名を更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンのホスト名を更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-hostname \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "new-example-host"
  }'
```

### マシンのメモを更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンのメモを更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-memo \
  -H "Content-Type: application/json" \
  -d '{
    "memo": "Updated memo for this machine"
  }'
```

### マシンのlast_aliveを更新する（PUT）

```bash
# このコマンドは指定されたIDのマシンのlast_aliveフィールドを現在の日時に更新します。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-last-alive
```

### マシンのCPUアーキテクチャを更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンのCPUアーキテクチャを更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-cpu_arch \
  -H "Content-Type: application/json" \
  -d '{
    "cpu_arch": "arm64"
  }'
```

### マシンのメモリサイズを更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンのメモリサイズを更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-memory_size \
  -H "Content-Type: application/json" \
  -d '{
    "memory_size": "16GB"
  }'
```

### マシンのディスク情報を更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンのディスク情報を更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-disk_info \
  -H "Content-Type: application/json" \
  -d '{
    "disk_info": "SSD, 500GB"
  }'
```

### マシンのOS名を更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンのOS名を更新します。例としてUUID=70ae9891-fc07-45b9-8364-3ab159ee2048を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/update-os_name \
  -H "Content-Type: application/json" \
  -d '{
    "os_name": "Ubuntu 22.04"
  }'
```

### インターフェースのMACアドレスを更新する（PUT）

```bash
# このコマンドは指定されたUUIDのマシンのインターフェースのMACアドレスを更新します。例としてマシンUUID=70ae9891-fc07-45b9-8364-3ab159ee2048、インターフェース名="eth0"を使用しています。
curl -X PUT http://localhost:3001/api/machines/70ae9891-fc07-45b9-8364-3ab159ee2048/interfaces/eth0/update-mac_address \
  -H "Content-Type: application/json" \
  -d '{
    "mac_address": "00:1a:2b:3c:4d:5e"
  }'
```

## ライセンス

このプロジェクトはMITライセンスの下で公開されています。詳細はLICENSEファイルをご覧ください。
