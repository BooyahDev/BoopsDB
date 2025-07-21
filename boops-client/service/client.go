package service

import (
    "boops-client/api"
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

const baseURL = "http://your-server/api/machines"

func Register(info api.MachineInfo) error {
    b, _ := json.Marshal(info)
    resp, err := http.Post(baseURL, "application/json", bytes.NewBuffer(b))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("status: %s", resp.Status)
    }
    return nil
}

func Fetch(id string) (api.MachineInfo, error) {
    var info api.MachineInfo
    url := fmt.Sprintf("%s/%s", baseURL, id)
    resp, err := http.Get(url)
    if err != nil {
        return info, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return info, fmt.Errorf("status: %s", resp.Status)
    }
    body, _ := io.ReadAll(resp.Body)
    json.Unmarshal(body, &info)
    return info, nil
}

