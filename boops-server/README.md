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

GET の `interfaces` は配列のままで、`ips` の各行にも `id` を返します。全マシン PUT の `interfaces` は従来の NIC 名をキーにした object です。NIC の value と IP の各要素に任意の `id` を送れます。ID 付きの NIC は名前を変更しても同じ行を更新し、IP も ID を維持して更新します。ID を送らない旧形式は NIC 名で対応付け、IP はアドレス・サブネットの一致と出現順で既存行に一度ずつ対応付けます。新しい行は最後に追加します。

全マシン PUT は NIC と IP の集合を完全に置き換えます。`interfaces` は必須です。空 object `{}` は全 NIC の削除を意味し、含めた NIC には 1 個以上の IP が必要です。省いた NIC と IP は削除されます。未知の ID、別マシン・別 NIC の ID、重複 ID、重複する最終 NIC 名、形式不正な要求は 400 です。既存 DB で同名 NIC が複数ある場合、名前だけによる更新・削除は 409 を返します。明示 ID を付けた全マシン PUT で一意の名前へ整理できます。マシンや対象 NIC が存在しない場合は 404 です。

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
