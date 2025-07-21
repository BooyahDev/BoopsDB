package cmd

import (
    "boops-client/service"
    "boops-client/system"
    "fmt"
    "github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Sync settings from server",
    Run: func(cmd *cobra.Command, args []string) {
        id := system.GetMachineID()
        info, err := service.Fetch(id)
        if err != nil {
            fmt.Println("取得失敗:", err)
            return
        }

        err = system.ApplyNetworkSettings(info)
        if err != nil {
            fmt.Println("適用失敗:", err)
        } else {
            fmt.Println("設定適用完了")
        }
    },
}

