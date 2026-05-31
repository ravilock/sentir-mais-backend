# sentir-mais-backend

Primeiro backend executável para o MVP do Sentir Mais, agora alinhado ao bootstrap do `finance-app-backend` com entrypoint em `cmd/` e composition root em `internal/api`.

## O que já existe

- API HTTP em Go com scaffold organizado em `cmd/` + `internal/`
- auth própria com:
  - `POST /auth/register`
  - `POST /auth/login`
  - `GET /auth/me`
- chat inicial com:
  - `POST /chats`
  - `POST /chats/{chatId}/messages`
  - `GET /chats/{chatId}/messages`
- `GET /dashboard/week` autenticado com payload inicial vazio
- resposta conversacional stubada atrás de uma interface de LLM
- middlewares básicos de request id, log, recover e CORS
- aliases versionados em `/api/v1/*` sem remover as rotas atuais

## Estado atual

O armazenamento ainda é em memória para manter o backend funcional enquanto a camada MongoDB não é ligada. A separação entre serviços, handlers e repositórios já deixa o caminho aberto para trocar essa implementação sem quebrar o contrato HTTP.

## Rodando localmente

```bash
go run ./cmd/sentir-mais-api
```

Variáveis opcionais:

- `HTTP_ADDRESS` (default `:8080`)
- `HTTP_ADDRESS` (default `:8001`)
- `SESSION_TTL_SECONDS` (default `604800`)
- `CORS_ALLOWED_ORIGINS` (CSV; default `http://localhost:3000,http://localhost:5173,http://localhost:4000`)

## Rotas

As rotas atuais continuam válidas sem prefixo e também estão expostas no formato versionado usado como referência no repositório de exemplo:

- legado: `/auth/*`, `/chats/*`, `/dashboard/week`
- versionado: `/api/v1/auth/*`, `/api/v1/chats/*`, `/api/v1/dashboard/week`
- healthcheck: `/healthz` e `/api/healthcheck`
