# boops-cli

BoopsDB API server を参照するGo製CLIです。

## Build

```bash
go build -o boops-cli .
```

## Usage

```bash
./boops-cli register
./boops-cli search
./boops-cli search -q "web01"
./boops-cli search -q "web01" -no-color
./boops-cli search "192.168.1"
./boops-cli get <machine-id>
./boops-cli <machine-id>
```

検索一覧には `ID`、`HOSTNAME`、`IP`、`PURPOSE`、仮想/実機の種別、最終更新の相対時間が表示されます。種別は `◆ VM` が仮想マシン、`■ HW` が実機です。

既定のAPI URLは `https://boopsdb-api.booyah.dev/api` です。別環境を見る場合は各コマンドに `-api` を渡してください。

```bash
./boops-cli search -api http://localhost:3001/api -q ubuntu
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
