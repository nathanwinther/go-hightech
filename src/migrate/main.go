package main

import (
    "fmt"
    "encoding/json"
    "io/ioutil"
    "os"
    "net/http"
    "path/filepath"
    "strconv"
    "hightech/config"
    "hightech/invoice"
)

func main() {
    switch len(os.Args) {
        case 1:
            break
        case 2:
            err := os.Chdir(os.Args[1])
            if err != nil {
                panic(err)
            }
            break
        default:
            return
    }

    err := config.Load("./data/data.db")
    if err != nil {
        panic(err)
    }

    allow, _ := strconv.ParseBool(config.Get("archive_migrate"))
    if allow != true {
        fmt.Println("Migrate not allowed in this environment")
        return
    }

    config.Dump(os.Stdout)

    m, err := getData()
    if err != nil {
        panic(err)
    }

    invoiceData := config.Get("invoice_data")
    fmt.Printf("=> %s\n", invoiceData)
    m.Invoice.Save(invoiceData)

    archivePath := config.Get("archive_path")
    err = os.RemoveAll(archivePath)
    if err != nil {
        panic(err)
    }

    err = os.MkdirAll(archivePath, 0755)
    if err != nil {
        panic(err)
    }

    for _, a := range m.Archive {
        err = saveFile(a.Url, filepath.Join(archivePath, a.Name))
        if err != nil {
            fmt.Println(err.Error())
        }
    }

    fmt.Println("OK")
}

func getData() (*invoice.Migrate, error) {
    url := config.Get("archive_migrate_url")

    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    b, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    m := new(invoice.Migrate)
    err = json.Unmarshal(b, m)
    if err != nil {
        return nil, err
    }

    return m, nil
}

func saveFile(url string, path string) error {
    fmt.Printf("<= %s\n", url)

    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    b, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    err = ioutil.WriteFile(path, b, 0644)
    if err != nil {
        return err
    }

    fmt.Printf("=> %s\n", path)

    return nil
}

