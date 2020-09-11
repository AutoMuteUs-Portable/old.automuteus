package main

import (
    "encoding/json"
    "io/ioutil"
    "log"
    "os"
    "bufio"
    "strings"
)

func load() error {
    // Open and parse the `.env`
    file, err := os.Open(".env")
    if err != nil {
        log.Fatal(err)
        return err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    Env := map[string]string{}
    for scanner.Scan() {
        line := scanner.Text()

        if !(strings.HasPrefix(line, "#")) {
            line := strings.Split(line, "=")
            if len(line) != 2 {
                continue
            }

            key := line[0]
            value := line[1]
            if key == "" {
                continue
            }
            Env[key] = value
        }
    }

    // Read bytes
    bytes, err := ioutil.ReadFile("diffenvs.json")
    if err != nil {
        log.Fatal(err)
        return err
    }

    var EnvMap map[string]([]map[string]string)

    json.Unmarshal(bytes, &EnvMap)

    if EnvMap == nil {
        return nil
    }

    for _, i := range EnvMap["automuteus"] {
        for key, value := range i {
            os.Setenv(key, Env[value])
        }
    }

    return nil
}