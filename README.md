# sentir-mais-backend

Primeiro backend executável para o MVP do Sentir Mais, agora alinhado ao bootstrap do `finance-app-backend` com entrypoint em `cmd/` e composition root em `internal/api`.

## O que já existe

- API HTTP em Go com scaffold organizado em `cmd/` + `internal/`
- auth própria com:
  - `POST /auth/register`
  - `POST /auth/login`
  - `GET /auth/me`
- chat inicial com:
  - `GET /chats`
  - `POST /chats`
  - `POST /chats/{chatId}/messages`
  - `GET /chats/{chatId}/messages`
- dashboard autenticado com:
  - `GET /dashboard/week`
  - `GET /dashboard/timeline`
- resposta conversacional stubada atrás de uma interface de LLM
- middlewares básicos de request id, log, recover e CORS
- aliases versionados em `/api/v1/*` sem remover as rotas atuais
- persistência em MongoDB para usuários, sessões, chats, mensagens, análises, resumos e dead letters
- pipeline assíncrono de análise com Redis Streams: o chat responde de forma síncrona, enquanto extração, classificação e atualização de dashboard rodam em background no mesmo processo do backend

## Rodando localmente

Para rodar tudo em containers:

```bash
docker compose up -d
```

Para rodar somente dependências em containers e a API local via Go:

```bash
make run-db
```

Depois rode a API local:

```bash
go run ./cmd/sentir-mais-api
```

Variáveis opcionais:

- `HTTP_ADDRESS` (default `:8001`)
- `SESSION_TTL_SECONDS` (default `604800`)
- `CORS_ALLOWED_ORIGINS` (CSV; default `http://localhost:3000,http://localhost:5173,http://localhost:4000`)
- `MONGO_URI` (default `mongodb://localhost:27017`)
- `MONGO_DATABASE` (default `sentir-mais`)
- `REDIS_ADDR` (default `localhost:6379`)
- `REDIS_PASSWORD` (default empty)
- `ANALYSIS_QUEUE_NAME` (default `analysis-jobs`)
- `PROMPTER_BASE_URL` (default empty; example `http://localhost:8020`)
- `PROMPTER_API_KEY` (default empty; sent as the `Authorization` header to the prompter)
- `PROMPTER_TIMEOUT_SECONDS` (default `30`)
- `CLASSIFIER_BASE_URL` (default empty; example `http://localhost:8010`)
- `CLASSIFIER_API_KEY` (default empty; sent as the `Authorization` header to the classifier)
- `CLASSIFIER_TIMEOUT_SECONDS` (default `30`)

O compose local sobe:

- MongoDB em `mongodb://localhost:27017`
- Redis em `localhost:6379`
- Mongo Express em `http://localhost:8081`
- Prompter em `http://localhost:8020`
- Classifier em `http://localhost:8010`
- Frontend em `http://localhost:3000`

O serviço `frontend` do compose do backend agora faz build local do projeto `../sentir-mais` e injeta `API_URL` em tempo de build do Vite. O default é:

- `FRONTEND_API_URL=http://localhost:8001/api/v1`

Se quiser apontar o frontend para outro backend antes do build:

```bash
FRONTEND_API_URL=http://localhost:8001/api/v1 docker compose up -d --build frontend
```

O serviço `classifier` agora aceita override por imagem:

- default: `ghcr.io/ravilock/sentir-mais-classifier:latest`

Para trocar a imagem, exporte `CLASSIFIER_IMAGE` antes de subir o compose.

Exemplo:

```bash
CLASSIFIER_IMAGE=ghcr.io/ravilock/sentir-mais-classifier:latest docker compose up -d classifier
```

Para usar o prompter com Ollama rodando no host:

```bash
export PROMPTER_LOCAL_LLM=true
export PROMPTER_DEFAULT_MODEL=qwen2.5:7b
docker compose up -d prompter
```

O compose já expõe `host.docker.internal` para o container do prompter e aponta `LLM_BASE_URL` para `http://host.docker.internal:11434` por default, então o único pré-requisito é ter o daemon do Ollama rodando no host.

Exemplo para preparar o host:

```bash
make run-ollama-host
```

Ou, sem `make`:

```bash
./scripts/run-ollama-host.sh
```

Para descarregar o modelo depois:

```bash
make stop-ollama-host
```

`make run-ollama-host` nao sobe um novo `ollama serve`. Ele:

- garante que o modelo exista com `ollama pull`
- faz preload do modelo no daemon ja rodando via `ollama run "<modelo>" ""`

`make stop-ollama-host` descarrega o modelo com `ollama stop <modelo>`.

Se quiser usar outro endpoint ou outro modelo local, sobrescreva:

```bash
export PROMPTER_LOCAL_LLM=true
export PROMPTER_LLM_BASE_URL=http://host.docker.internal:11434
export PROMPTER_DEFAULT_MODEL=llama3.1:8b
docker compose up -d prompter
```

O script aceita overrides por ambiente:

```bash
OLLAMA_MODEL=llama3.1:8b OLLAMA_PULL_MODEL=true ./scripts/run-ollama-host.sh
```

Para usar o prompter e o classifier do compose com a API rodando localmente:

```bash
export PROMPTER_BASE_URL=http://localhost:8020
export PROMPTER_API_KEY=sentir-mais-local-prompter-key
export CLASSIFIER_BASE_URL=http://localhost:8010
export CLASSIFIER_API_KEY=sentir-mais-local-classifier-key
```

Se `PROMPTER_BASE_URL` estiver configurada, o backend usa `sentir-mais-prompter` para gerar as respostas conversacionais. O mesmo cliente também é usado pelo worker para extração estruturada quando houver contexto suficiente.

Se `CLASSIFIER_BASE_URL` estiver configurada, o worker classifica mensagens de usuário de forma assíncrona e persiste o resultado em `message_analyses`.

## Pipeline assíncrono de análise

`POST /chats` e `POST /chats/{chatId}/messages` persistem a conversa e retornam a resposta do assistente sem esperar extração, classificação ou atualização de dashboard. Depois de persistir a mensagem do usuário, o backend tenta publicar um job em Redis Streams. Essa publicação é best-effort: se Redis estiver indisponível, o erro é logado e a resposta do chat continua sendo retornada ao usuário.

O worker roda no mesmo processo Go da API. Ele:

- consome jobs de Redis Streams com consumer group
- recupera jobs pendentes via `XAUTOCLAIM`
- usa lock por chat para evitar processamento paralelo do mesmo chat
- reconstrói o histórico autoritativo a partir do MongoDB
- executa extração, classificação, persistência da análise e atualização dos resumos diário/semanal
- grava falhas terminais em `analysis_dead_letters`

Por causa disso, os endpoints de dashboard (`/dashboard/week` e `/dashboard/timeline`) são eventualmente consistentes. Logo após enviar uma mensagem, os dados de humor/eventos podem ainda não refletir a última conversa até o worker concluir o job.

## Rotas

As rotas atuais continuam válidas sem prefixo e também estão expostas no formato versionado usado como referência no repositório de exemplo:

- legado: `/auth/*`, `/chats`, `/chats/{chatId}/messages`, `/dashboard/week`, `/dashboard/timeline`
- versionado: `/api/v1/auth/*`, `/api/v1/chats`, `/api/v1/chats/{chatId}/messages`, `/api/v1/dashboard/week`, `/api/v1/dashboard/timeline`
- healthcheck: `/healthz` e `/api/healthcheck`
