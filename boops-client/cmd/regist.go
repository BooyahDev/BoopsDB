package cmd

import (
    "boops-client/service"
    "boops-client/system"
    "fmt"
    "github.com/spf13/cobra"
)

var registCmd = &cobra.Command{
    Use:   "regist [id]",
    Short: "Register machine info to server",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        id := args[0]
        info := system.CollectSystemInfo(id)
        err := service.Register(info)
        if err != nil {
            fmt.Println("登録失敗:", err)
        } else {
            fmt.Println("登録完了")
        }
    },
}

