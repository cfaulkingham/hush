# shop-api

A tiny backend that needs three secrets. It never reads `.env` itself — the process
environment is the only input. That is the hush daily loop.

```bash
# from the repo root
go build -o hush ./cmd/hush

cd samples/shop-api
cp .env.example .env          # you already live this way
../../hush init
../../hush import .env
rm .env                       # plaintext can go away
../../hush run -- python3 app.py
../../hush run -- python3 app.py serve
```
