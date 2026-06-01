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
- persistência em MongoDB para usuários, sessões, chats e mensagens

## Rodando localmente

Suba as dependencias locais:

```bash
docker compose up -d
```

Para subir o classifier com GPU NVIDIA:

```bash
docker compose up -d classifier
```

Ou:

```bash
make run-db-gpu
```

Depois rode a API:

```bash
go run ./cmd/sentir-mais-api
```

Variáveis opcionais:

- `HTTP_ADDRESS` (default `:8001`)
- `SESSION_TTL_SECONDS` (default `604800`)
- `CORS_ALLOWED_ORIGINS` (CSV; default `http://localhost:3000,http://localhost:5173,http://localhost:4000`)
- `MONGO_URI` (default `mongodb://localhost:27017`)
- `MONGO_DATABASE` (default `sentir-mais`)
- `PROMPTER_BASE_URL` (default empty; example `http://localhost:8020`)
- `PROMPTER_API_KEY` (default empty; sent as the `Authorization` header to the prompter)
- `PROMPTER_TIMEOUT_SECONDS` (default `10`)
- `CLASSIFIER_BASE_URL` (default empty; example `http://localhost:8010`)
- `CLASSIFIER_API_KEY` (default empty; sent as the `Authorization` header to the classifier)
- `CLASSIFIER_TIMEOUT_SECONDS` (default `10`)

O compose local sobe:

- MongoDB em `mongodb://localhost:27017`
- Mongo Express em `http://localhost:8081`
- Prompter em `http://localhost:8020`
- Classifier em `http://localhost:8010`

O serviço `classifier` agora aceita override por imagem:

- default: `ghcr.io/ravilock/sentir-mais-classifier:latest-gpu`
- CPU: `ghcr.io/ravilock/sentir-mais-classifier:latest`

Para trocar a variante, exporte `CLASSIFIER_IMAGE` antes de subir o compose.

Exemplo para forçar CPU:

```bash
CLASSIFIER_IMAGE=ghcr.io/ravilock/sentir-mais-classifier:latest docker compose up -d classifier
```

Pré-requisitos no host:

- driver NVIDIA instalado
- `nvidia-container-toolkit` configurado no Docker

Para usar o prompter e o classifier do compose com a API rodando localmente:

```bash
export PROMPTER_BASE_URL=http://localhost:8020
export PROMPTER_API_KEY=sentir-mais-local-prompter-key
export CLASSIFIER_BASE_URL=http://localhost:8010
export CLASSIFIER_API_KEY=sentir-mais-local-classifier-key
```

Se `PROMPTER_BASE_URL` estiver configurada, o backend usa `sentir-mais-prompter` para gerar as respostas conversacionais.

Se `CLASSIFIER_BASE_URL` estiver configurada, o backend classifica cada mensagem do usuario e persiste o resultado em `message_analyses`.

## Rotas

As rotas atuais continuam válidas sem prefixo e também estão expostas no formato versionado usado como referência no repositório de exemplo:

- legado: `/auth/*`, `/chats/*`, `/dashboard/week`
- versionado: `/api/v1/auth/*`, `/api/v1/chats/*`, `/api/v1/dashboard/week`
- healthcheck: `/healthz` e `/api/healthcheck`
