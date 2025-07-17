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
- `ip_address`: IPアドレス
- `subnet_mask`: サブネットマスク
- `gateway`: ゲートウェイ
- `dns_servers`: DNSサーバーのリスト（カンマ区切り文字列形式で保存。APIでは配列として取得・送信）

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
    "interfaces": {
      "eth0": { "ip_address": "192.168.1.1", "subnet_mask": "255.255.255.0", "gateway": "192.168.1.254", "dns_servers": ["8.8.8.8", "8.8.4.4"] },
      "lo": { "ip_address": "127.0.0.1", "subnet_mask": "255.0.0.0", "gateway": "", "dns_servers": [] }
    }
  }'
```

### マシンを更新する（PUT）

```bash
# このコマンドは指定されたIDのマシン情報を更新します。例としてID=1を使用しています。
curl -X PUT http://localhost:3001/api/machines/1 \
  -H "Content-Type: application/json" \
  -d '{
    "hostname": "updated-example",
    "model_info": "Updated Model",
    "usage_desc": "Updated usage description",
    "memo": "This is an updated example machine",
    "interfaces": {
      "eth0": { "ip_address": "192.168.1.1", "subnet_mask": "255.255.255.0", "gateway": "192.168.1.254", "dns_servers": ["8.8.8.8", "8.8.4.4"] },
      "lo": { "ip_address": "127.0.0.1", "subnet_mask": "255.0.0.0", "gateway": "", "dns_servers": [] }
    }
  }'
```

### マシンを削除する（DELETE）

```bash
# このコマンドは指定されたIDのマシンとそのインターフェースをAPIサーバーから削除します。例としてID=1を使用しています。
curl -X DELETE http://localhost:3001/api/machines/1
```

## ライセンス

このプロジェクトはMITライセンスの下で公開されています。詳細はLICENSEファイルをご覧ください。
