# wsbot (minimal WS client with JSON store)

## Run (Linux)

```bash
cd wsbot
go mod tidy
# Terminal 1: start mock server
go run ./cmd/mockserver
# Terminal 2: start client
go run ./cmd/wsbot --config ./config.yaml
```

Messages "上班" or "下班" will append events into `data/worklog.json`.
