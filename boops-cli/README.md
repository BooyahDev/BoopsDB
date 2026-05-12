# boops-cli

BoopsDB API server を参照するGo製CLIです。

## Build

```bash
go build -o boops .
```

## Usage

```bash
boops register
boops "web01"
boops "192.168.1"
boops -id <machine-id>
boops "ubuntu" -sort hostname -order asc
boops "ubuntu" -no-color
boops search
boops search -q "web01"
boops search -q "web01" -no-color
boops search "192.168.1"
boops get <machine-id>
```

検索一覧には `ID`、`HOSTNAME`、`IP`、仮想/実機の種別、最終更新の相対時間、`PURPOSE` が表示されます。`PURPOSE` は一番右に表示されます。種別は `◆ VM` が仮想マシン、`■ HW` が実機です。

検索結果はデフォルトで最終更新日時の新しい順です。`-sort` と `-order` で並び替えできます。

```bash
boops "ubuntu" -sort updated_at -order desc
boops "ubuntu" -sort hostname -order asc
boops "ubuntu" -sort purpose -order asc
```

既定のAPI URLは `https://boopsdb-api.booyah.dev/api` です。別環境を見る場合は各コマンドに `-api` を渡してください。

```bash
boops -api http://localhost:3001/api ubuntu
```

## Install

GitHub Releaseから環境に合う成果物を取得して、実行コマンド `boops` としてインストールできます。

Linux / macOS:

```bash
curl -fsSL https://github.com/BooyahDev/BoopsDB/releases/latest/download/install.sh | sh
```

Windows PowerShell:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -Command "iwr https://github.com/BooyahDev/BoopsDB/releases/latest/download/install-windows.ps1 -OutFile install-windows.ps1; .\install-windows.ps1"
```

特定バージョンを入れる場合:

```bash
BOOPS_VERSION=boops-cli/v0.1.0 sh install.sh
```

```powershell
.\install-windows.ps1 -Version boops-cli/v0.1.0
```

## Release build

`boops-cli/v*` または `v*` のタグをpushすると、GitHub ActionsがWindows、macOS、Linux向けにamd64、386、arm64、Linux armv7の成果物をビルドし、GitHub Releaseへ添付します。

### Release steps

例として `boops-cli/v0.1.0` をリリースする場合:

```bash
git switch main
git pull
git status
git tag -a boops-cli/v0.1.0 -m "boops-cli v0.1.0"
git push origin boops-cli/v0.1.0
```

タグをpushすると `.github/workflows/boops-cli-release.yml` が起動します。Actionsが成功すると、GitHub Releases に各プラットフォーム向けの成果物が保存されます。

タグを付け直したい場合は、既存のReleaseとタグを確認してから行ってください。

```bash
git tag -d boops-cli/v0.1.0
git push origin :refs/tags/boops-cli/v0.1.0
git tag -a boops-cli/v0.1.0 -m "boops-cli v0.1.0"
git push origin boops-cli/v0.1.0
```
