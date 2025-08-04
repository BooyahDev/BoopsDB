#!/bin/bash

# ユーザーから機械IDを入力してもらう
read -p "Please enter the machine ID: " MACHINE_ID

# cronジョブを設定するための内容
CRON_JOB="* * * * * root curl -X PUT https://boopsdb-api.booyah.dev/api/machines/$MACHINE_ID/update-last-alive"

# /etc/cron.d/boops ファイルにcronジョブを上書きで追加
echo "$CRON_JOB" | sudo tee /etc/cron.d/boops

echo "Cron job added successfully!"
