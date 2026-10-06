# Boops Server

Boops server manages machine information, including network interfaces with multiple IP addresses.

## Database Schema

### Tables:

1. `machines`: Stores machine information
   - id: UUID (Primary Key)
   - hostname: Machine hostname
   - model_info: Machine model information
   - usage_desc: Usage description
   - memo: Notes about the machine
   - purpose: Purpose of the machine
   - last_alive: Last known alive timestamp
   - cpu_info: CPU information
   - cpu_arch: CPU architecture
   - memory_size: Memory size
   - disk_info: Disk information
   - os_name: Operating system name
   - is_virtual: Flag indicating if this is a virtual machine
   - parent_machine_id: UUID of the parent machine (for virtual machines)
   - created_at: Creation timestamp
   - updated_at: Last update timestamp

2. `interfaces`: Stores network interfaces for each machine
   - id: Auto-incrementing ID (Primary Key)
   - machine_id: Machine UUID (Foreign Key to machines.id)
   - name: Interface name
   - gateway: Gateway IP address
   - dns_servers: Comma-separated list of DNS servers
   - mac_address: MAC address

3. `interface_ips`: Stores IP addresses and subnet masks for each interface
   - id: Auto-incrementing ID (Primary Key)
   - interface_id: Interface ID (Foreign Key to interfaces.id)
   - ip_address: IP address
   - subnet_mask: Subnet mask

## API Endpoints

### Machines:

- GET `/api/machines`: Get all machines with their interfaces and IP addresses
- POST `/api/machines`: Create a new machine with interfaces and IP addresses
- PUT `/api/machines/:id`: Update an existing machine and its interfaces/IPs
- DELETE `/api/machines/:id`: Delete a machine and all its interfaces/IPs

### Interfaces:

- POST `/api/machines/:id/interfaces`: Add a new interface to a machine with multiple IPs
- DELETE `/api/machines/:machineId/interfaces/:interfaceName`: Remove an interface from a machine
- PUT `/api/interfaces/:machineId/:interfaceName/ips`: Update IP addresses for an interface

## Data Format Examples

### Create Machine:

```json
{
  "hostname": "test-machine",
  "model_info": "Model XYZ",
  "usage_desc": "Testing",
  "memo": "Test machine for development",
  "purpose": "Development",
  "cpu_info": "Intel i7",
  "memory_size": "16GB",
  "os_name": "Ubuntu 20.04",
  "is_virtual": true,
  "parent_machine_id": "550e8400-e29b-41d4-a716-446655440000",
  "interfaces": {
    "eth0": {
      "ips": [
        { "ip_address": "192.168.1.1", "subnet_mask": "255.255.255.0" },
        { "ip_address": "10.0.0.1", "subnet_mask": "255.0.0.0" }
      ],
      "gateway": "192.168.1.254",
      "dns_servers": ["8.8.8.8", "8.8.4.4"],
      "mac_address": "00:11:22:33:44:55"
    }
  }
}
```

### Add Interface to Machine:

```json
{
  "name": "eth1",
  "ips": [
    { "ip_address": "192.168.2.1", "subnet_mask": "255.255.255.0" },
    { "ip_address": "172.16.0.1", "subnet_mask": "255.240.0.0" }
  ],
  "gateway": "192.168.2.254",
  "dns_servers": ["1.1.1.1"],
  "mac_address": "aa:bb:cc:dd:ee:ff"
}
```

## ネットワーク設定の API 契約

一覧と並べ替えを指定しない検索は、登録日時の昇順、同じ日時ではマシン ID の昇順で返します。登録日時が NULL の行は最後です。検索で明示した並べ替え条件は維持し、同値の場合は ID の昇順にします。NIC と IP はそれぞれ DB の ID の昇順で返すため、名前や設定を編集しても表示順を維持できます。

`GET /api/machines/search` は、従来どおり検索語 `q` が空欄なら空の `results` と `total: 0` を返します。任意の `all=1` を指定すると、空欄のまま全マシンを検索対象にできます。例: `/api/machines/search?all=1&limit=20&offset=20`。NIC がないマシンも含め、通常の `sort`、`order`、`limit`、`offset` と同じ並べ替え・ページ分割を使い、`results` / `pagination` の response 形は変わりません。`q` が非空の場合は `all=1` を指定してもその検索条件を使います。

GET の `interfaces` は配列のままで、`ips` の各行にも `id` を返します。全マシン PUT の `interfaces` は従来の NIC 名をキーにした object です。NIC の value と IP の各要素に任意の `id` を送れます。ID 付きの NIC は名前を変更しても同じ行を更新し、IP も ID を維持して更新します。ID を送らない旧形式は NIC 名で対応付け、IP はアドレス・サブネットの一致と出現順で既存行に一度ずつ対応付けます。新しい行は最後に追加します。

全マシン PUT は NIC と IP の集合を完全に置き換えます。`interfaces` は必須です。空 object `{}` は全 NIC の削除を意味し、含めた NIC には 1 個以上の IP が必要です。省いた NIC と IP は削除されます。未知の ID、別マシン・別 NIC の ID、重複 ID、重複する最終 NIC 名、形式不正な要求は 400 です。既存 DB で同名 NIC が複数ある場合、名前だけによる更新・削除は 409 を返します。明示 ID を付けた全マシン PUT で一意の名前へ整理できます。マシンや対象 NIC が存在しない場合は 404 です。

NIC 名は大文字・小文字を区別します。`eth0` と `ETH0` は別の NIC として追加・更新・削除でき、ID を省いた全マシン PUT でも厳密に一致する名前へ対応付けます。DB の照合順序にかかわらず同じ名前判定を使い、完全に同じ名前の既存行が複数ある場合だけ 409 を返します。

非空 gateway はマシンごとに最大 1 つです。空文字、`null`、空白だけの文字列、`0.0.0.0` は空文字に揃え、GET でも空文字を返します。非空値には IPv4 アドレスが必要です。全マシン POST・PUT で複数の非空値を送ると 400 になり、部分保存しません。

既存の `PUT /api/interfaces/:machineId/:interfaceName/update-gateway` は `{ "gateway": "10.0.0.1" }` を受け付け、成功時は `{ "message": "Gateway updated" }` を返します。非空値を送ると同じマシンの他 NIC の gateway を同時に解除し、空値を送ると対象 NIC だけを解除します。対象があれば同じ値の再送も成功します。NIC 追加 POST の非空 gateway も同じ選択操作です。各更新は machine 行をロックしてから子行を処理し、並行した選択と更新を直列化します。検証・SQL エラー時はトランザクション全体を rollback し、接続を解放します。

既存 DB に複数の非空 gateway がある場合、GET はそのまま各値を返します。読み取りだけで勝手に値を選びません。gateway 専用 PUT で使用する NIC を選ぶと、他の値を解除できます。

## テスト

`npm test` は単体・HTTP factory・DB 結合テストを実行します。`httpApp.js` の `createApp(database)` は listener や production DB 設定を読み込みません。DB 設定を指定しない場合、DB 結合テストだけを skip します。`npm run test:unit` は DB なしの検証用です。

結合テストには、本番と分離した loopback の MySQL と `boops_test_` で始まる専用 DB が必要です。テスト開始時にその DB の 3 テーブルを作り直します。production の `.env` は使いません。Docker が使用できる場合、`npm run test:db` で新しい使い捨てコンテナを起動できます。表示された接続情報を設定して実行してください。

```sh
TEST_DB_HOST=127.0.0.1 TEST_DB_PORT=33306 \
TEST_DB_NAME=boops_test_console TEST_DB_USER=root \
TEST_DB_PASSWORD=boops-disposable-test npm test
```

既に隔離した MySQL がある場合も、同じ `TEST_DB_*` で接続できます。結合テストは ID 維持、旧形式、空値・同値更新、並行 gateway 選択、曖昧な NIC 名、登録日時が NULL の順序、SQL 失敗後の全値復元を実 DB で確認します。

## GitHub への Markdown 退避

`.env.archive.example` の設定を `.env` に追加します。`GITHUB_ARCHIVE_ENABLED=true` とトークンの両方が設定された場合だけ有効です。フラグ未設定・`false`・その他の値、または `GITHUB_ARCHIVE_TOKEN` が空なら、更新時退避・起動時同期・定期チェック・再試行はすべて無効です。Node.js 22（既存 Dockerfile と同じ）を使用してください。

```dotenv
GITHUB_ARCHIVE_ENABLED=true
GITHUB_ARCHIVE_TOKEN=github_pat_...
GITHUB_ARCHIVE_REPOSITORY=BooyahDev/BoopsDB-Archive
GITHUB_ARCHIVE_BRANCH=
GITHUB_ARCHIVE_DIR=.boops-archive
```

トークンには対象プライベートリポジトリの **Contents: Read and write** 権限を付与してください。組織の承認が必要な場合は承認済みトークンを使用します。SSH URL で指定された同じリポジトリに、トークン認証の GitHub Contents API でコミットを作成します。SSH キー・git コマンド・追加 npm 依存は不要です。空のリポジトリも最初のコミットで初期化します。ブランチ未指定はリポジトリの既定ブランチを使用し、既存の別ブランチを使用するときだけ指定してください。

有効なインスタンスでは起動時に全件退避し、その後はマシン・NIC・IP 関連の POST / PUT / DELETE が成功したときに非同期で退避します。短時間の更新は 2 秒でまとめ、`README.md` のマシン・ハードウェア、NIC・ネットワーク、IP を統合した 1 つの表を全件更新します。マシン ID、ホスト名、モデル、用途・説明・メモ、CPU・アーキテクチャ・メモリ・ディスク・OS、仮想マシン区分・親 ID、NIC ID・名前・MAC・gateway・DNS、IP ID・アドレス・サブネット・DNS 登録設定を含みます。1 行につき 1 つの IP 設定を掲載し、複数の NIC・IP がある場合は各行にマシン情報を繰り返して対応関係を示します。NIC がないマシン・IP がない NIC も含み、削除されたマシン・NIC・IP は現在の表から除去します（過去のコミットには残ります）。

`update-last-alive` は退避を一切起動せず、`last_alive`・`created_at`・`updated_at` は表にも差分判定にも含みません。全マシン PUT でも構成が同一なら GitHub 通信を省略します（起動後の初回はリモートとの比較が必要）。読み取り・失敗した API は退避を起動しません。既存の保存・検証・HTTP 応答は維持し、GitHub の失敗で API を失敗させません。

退避前に `GITHUB_ARCHIVE_DIR/README.md` を atomic rename で保存します。GitHub 障害時は 60 秒から最大 1 時間までの指数バックオフで再試行し、RateLimit の Retry-After / リセット時刻にも従います。再試行時は最新 DB を読み直し、DB が使えない場合は保存済み Markdown を送信します。プロセス再起動後も起動時同期で再送します。ログの `GitHub archive failed; retry scheduled` で失敗を確認できます。

有効な API の起動から **24 時間ごと（1 日 1 回）**に、最新 DB と GitHub の README を比較します。ローカルで同一内容と判定されていても、この定期チェックでは GitHub を読み直し、差分がある場合だけコミットします。他の API インスタンス経由の更新や、GitHub 側での変更・削除もこのチェックで同期します。更新時退避・定期チェック・再試行は同じワーカーで直列実行します。失敗時は回数・期間の上限なく再試行を続けるため、1 時間以上の GitHub 障害からも復旧後に追いつきます。1 時間は再試行の最大間隔で、打ち切り時間ではありません。RateLimit で指定された待機時間が長い場合はそれを優先します。再起動すると 24 時間のタイマーを再設定しますが、起動時にも全件同期します。

Docker / Kubernetes ではこのディレクトリを永続ボリュームに配置し、`.env` または Secret でトークンを渡してください。退避担当の **1 台だけ**に `GITHUB_ARCHIVE_ENABLED=true`、その他の全 API インスタンスには `GITHUB_ARCHIVE_ENABLED=false` を設定してください。複数インスタンスでは共有ロックを実装していないため、古いスナップショットによる上書きを防げません。トークンや実データ入りのローカル Markdown はソースリポジトリへコミットしないでください。

非同期処理のため、DB 保存直後から退避までに短い遅延があります。退避前にプロセスと MySQL の両方が失われた場合、その未退避分は復元できません。MySQL 障害時には GitHub の README を閲覧でき、API の DB 読み取り動作を切り替える機能は追加していません。

仕様: [GitHub Contents API](https://docs.github.com/en/rest/repos/contents)。`npm run test:unit` で Markdown・GitHub モック・再試行・HTTP の既存応答と Heartbeat 除外を検証できます。

### 明示的な同期と Docker の確認

起動ログに `GitHub archive enabled` が出れば退避が有効です。`disabled` の場合は理由を確認してください。成功時は `GitHub archive sync completed`、失敗時は処理段階と GitHub の HTTP ステータスをログに出します。トークンや DB の内容はログに出しません。

Compose の `env_file: .env` はコンテナ作成時に環境変数を渡します。設定変更後はコンテナを作り直し、コード変更後はイメージも再ビルドしてください。

```sh
docker compose up -d --build --force-recreate boops-server
```

実行中の API に対して、次の操作で最新 DB と GitHub の比較・更新を要求できます。既存ワーカーにキューするため、通常の更新や再試行と競合しません。障害時のバックオフ待機は維持します。既存 Dockerfile のコンテナ内には API の Node プロセスが 1 個ある想定です。

```sh
docker compose exec boops-server sh -c 'kill -USR2 $(pidof node)'
docker compose logs -f boops-server
```

単発実行用に `npm run archive:sync` もあります。成功時は終了コード 0、失敗・無効時は 1 です。同じリポジトリへの並行書き込みを避けるため、API の退避ワーカーを止めてから実行してください。下記の実行中はこの API インスタンスが停止します。

```sh
docker compose stop boops-server
docker compose run --rm --no-deps boops-server npm run archive:sync
docker compose up -d boops-server
```

`dotenv` の `injecting env (0)` は、Compose がすでに変数を渡していて `.env` から追加で注入した変数が 0 件の場合にも表示されます。この表示だけではトークン不足とは判断できません。

## Notion への退避

GitHub に加え、Notion API でも同じ構成情報をアーカイブできます。GitHub と独立した設定・再試行ワーカーを使用します。片方の障害で他方の同期を停止しません。起動時、構成変更が成功した後、24 時間ごとの比較、SIGUSR2 と `npm run archive:sync` による明示同期は、すべて有効な退避先に適用されます。Heartbeat・日時情報の除外、同一内容の送信省略、ローカル保存と長時間障害時の再試行も共通です。

```dotenv
NOTION_ARCHIVE_ENABLED=true
NOTION_ARCHIVE_TOKEN=ntn_...
NOTION_ARCHIVE_PAGE_ID=3f1f663606fc8061a287f4838ff648c3
NOTION_ARCHIVE_DIR=.boops-archive/notion
```

Notion インテグレーションにコンテンツの読み取り・挿入・更新権限を付与し、指定ページにその接続を追加してください。`NOTION_ARCHIVE_PAGE_ID` はページ URL の `/p/` の ID です。`?v=` はビュー ID なので使用しません。通常ページかデータベースかを API で自動判定します。データベースの場合は、その中に `BoopsDB Archive [managed]` という 1 件のページを作成・再利用し、その本文へ統合表を保存します。既存の DB プロパティ・他のレコードは変更しません。複数データソースを持つデータベースは対象を自動選択せず失敗します。その場合は退避用の通常ページを作り、その ID を設定してください。

指定ページ内の `BoopsDB Archive [managed]` トグルに、GitHub と同じ列・行を持つ **1 つのネイティブ表**を作成します。トグルを開いて閲覧してください。既存の他の本文・ブロックは変更しません。この名前と `BoopsDB Archive [managed] [updating]` は退避処理専用です。管理対象の表を手動編集しても、日次確認で DB の内容へ戻します。

新しい表を別のトグルで完成させてから前回の表を削除します。更新中の障害では前回の完成済み表を維持し、次回同期で未完成トグルや重複を整理します。更新中は一時的に前回と新規のトグルが並びます。Notion には GitHub のコミット履歴に相当する独自世代管理は追加していません。

100 行・リクエストサイズの制限に合わせて送信を分割し、長いセルもテキスト要素に分割します。通常はリクエストを約 350 ms ごとに送り、HTTP 429 / 529 などの `Retry-After` を再試行時に尊重します。セルが 100 テキスト要素または 1 行が安全な送信サイズを超える場合は切り捨てずに同期を失敗させ、前回の完成済み表を保持します。失敗ログの `notion-sync` と HTTP ステータスを確認してください。

複数台運用では Notion も 1 台だけ `NOTION_ARCHIVE_ENABLED=true` にします。他のインスタンスは `false`（未設定も無効）にしてください。既存の `.env` に設定を追加し、再ビルド・再作成します。

```sh
docker compose up -d --build --force-recreate boops-server
```

仕様: [Notion ブロック・表](https://developers.notion.com/reference/block)、[Notion API の制限](https://developers.notion.com/reference/request-limits)。トークンや実データはソースリポジトリへコミットしないでください。
