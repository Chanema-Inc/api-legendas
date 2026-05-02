## Visao Geral

Este projeto expoe uma API Go enxuta que recebe URLs de legendas, valida com seguranca o conteudo baixado, armazena a ultima legenda valida em cache e a serve para os clientes.

Principais capacidades:

- Aceitar URLs de legenda via POST /legenda
- Retornar a ultima legenda valida via GET /legenda
- Aceitar apenas extensoes de arquivo de legenda: .srt, .vtt, .webvtt
- Rejeitar conteudo malicioso de legenda
- Armazenar e servir o conteudo da legenda em cache com expiracao
- Aplicar limites de tamanho de arquivo
- Aplicar limitacao de taxa
- Restringir acesso entre origens com CORS configuravel por endpoint

## Requisitos

- Go 1.24.2
- Docker, se voce quiser executar a imagem de container

## Configuracao

A aplicacao carrega configuracao nesta ordem:

1. Variaveis de ambiente do processo
2. Valores de .env.prod quando APP_ENV=production e o valor nao estiver vazio
3. Valores de contingencia em .env.dev
4. Defaults internos quando necessario

Configuracoes disponiveis:

- APP_ENV: development ou production
- APP_PORT: porta do servidor HTTP
- STORAGE_BACKEND: memory_cache ou redis
- UPSTASH_REDIS_URL: URL completa do Upstash Redis
- REDIS_ADDR: host e porta do Redis
- REDIS_PASSWORD: senha do Redis
- REDIS_DB: numero do banco logico do Redis
- REDIS_KEY_PREFIX: prefixo aplicado as chaves no Redis
- MAX_SUBTITLE_SIZE_BYTES: tamanho maximo aceito para o payload da legenda
- ALLOWED_ORIGINS_LEGENDA_GET: dominios permitidos para GET /legenda (ex.: Cytube)
- ALLOWED_ORIGINS_LEGENDA_POST: dominios permitidos para POST /legenda (ex.: Fly.io)
- ALLOWED_ORIGINS_HEALTH_GET: dominios permitidos para GET /health (ex.: OnRender)
- CACHE_TTL: duracao de expiracao do cache (default: 5h)
- RATE_LIMIT_BURST: quantidade de requisicoes permitidas por janela por cliente
- RATE_LIMIT_WINDOW: duracao da janela de rate limit

Valores padrao de desenvolvimento ficam em .env.dev. Modelos de producao ficam em .env.prod.

## Exemplo De CORS

Configuracao sugerida para seu caso:

- ALLOWED_ORIGINS_LEGENDA_GET=https://cytube.com
- ALLOWED_ORIGINS_LEGENDA_POST=https://api.fly.io
- ALLOWED_ORIGINS_HEALTH_GET=https://seu-servico.onrender.com

Para liberar testes locais, adicione tambem:

- http://localhost:3000
- http://127.0.0.1:3000

Exemplo:

- ALLOWED_ORIGINS_LEGENDA_GET=https://cytube.com,http://localhost:3000,http://127.0.0.1:3000
- ALLOWED_ORIGINS_LEGENDA_POST=https://api.fly.io,http://localhost:3000,http://127.0.0.1:3000
- ALLOWED_ORIGINS_HEALTH_GET=https://seu-servico.onrender.com,http://localhost:3000,http://127.0.0.1:3000

## Execucao Local

```bash
go run .
```

## Execucao De Testes

```bash
go test ./... -count=1
```

## Endpoints Da API

### GET /health

Retorna uma resposta simples de verificacao de vida.

Resposta de sucesso:

```json
{
  "status": "ok"
}
```

### POST /legenda

Armazena uma legenda referenciada por URL.

Requisicao:

```json
{
  "url": "https://example.com/legenda/movie.srt"
}
```

Resposta de sucesso:

```json
{
  "id": "c8f8cc0f5b1d4e33ae52e2a763c4e81d",
  "url": "https://example.com/legenda/movie.srt"
}
```

### GET /legenda

Retorna a ultima legenda valida armazenada em cache, incluindo o conteudo da legenda.

Resposta de sucesso:

```json
{
  "id": "c8f8cc0f5b1d4e33ae52e2a763c4e81d",
  "url": "https://example.com/legenda/movie.srt",
  "content": "1\n00:00:00,000 --> 00:00:01,500\nHello\n"
}
```

Erros possiveis:

- `404 Not Found`: nenhuma legenda valida esta armazenada no momento, ou a legenda em cache expirou
- `403 Forbidden`: origem nao permitida pelo CORS
- `429 Too Many Requests`: rate limit excedido
- `500 Internal Server Error`: erro interno

## Regras De Validacao De Legenda

- URLs devem terminar com `.srt`, `.vtt` ou `.webvtt`
- Marcadores maliciosos como `<script`, `javascript:`, `<iframe`, `onerror=` e `onload=` sao rejeitados
- O corpo da legenda baixada nao pode estar vazio
- O servico valida o conteudo baixado, mas nao serve o corpo da legenda para clientes

## Estrategia De Cache

A implementacao atual suporta dois backends de cache:

- `memory_cache`: cache em memoria padrao para desenvolvimento local e implantacoes de instancia unica
- `redis`: backend de cache distribuido para implantacoes com multiplas instancias

Estrategias suportadas hoje:

- Expiracao por `CACHE_TTL`
- Limpeza automatica de entradas expiradas durante operacoes de leitura e escrita no backend em memoria
- Suporte explicito a invalidacao na implementacao de cache em memoria para futuros fluxos administrativos
- Replicacao da ultima legenda no Redis por meio de uma chave dedicada com TTL

A API depende de uma interface de armazenamento (`service.Store`), entao o backend pode ser substituido com impacto limitado no codigo dos handlers.

## CORS

O acesso entre origens e controlado por `ALLOWED_ORIGINS` e, opcionalmente, por allowlists especificas de metodo. A API:

- Aceita requisicoes preflight de origens permitidas
- Rejeita requisicoes de origens fora da allowlist do metodo efetivo (GET/POST)
- Retorna `Access-Control-Allow-Origin` apenas para origens aceitas

## Limitacao De Taxa

A limitacao de taxa e aplicada por identificador de cliente derivado do endereco remoto da requisicao.

Notas de implementacao:

- Implementacao: `RateLimiter` em `internal/httpapi/rate_limiter.go`

Notas de comportamento:

- `/health` e excluida da limitacao de taxa de requisicoes.
- Requisicoes para `/legenda` recebem limitacao de taxa por `RemoteAddr`.

Configuracao:

- `RATE_LIMIT_BURST`: numero maximo de requisicoes na janela configurada
- `RATE_LIMIT_WINDOW`: duracao da janela de reset

## Docker

Build da imagem:

```bash
docker build -t subtitle-delivery:local .
```

Executar o container:

```bash
docker run --rm -p 8080:8080 \
  -e APP_ENV=development \
  -e ALLOWED_ORIGINS_LEGENDA_GET=https://cytube.com,http://localhost:3000 \
  -e ALLOWED_ORIGINS_LEGENDA_POST=https://api.fly.io,http://localhost:3000 \
  -e ALLOWED_ORIGINS_HEALTH_GET=https://seu-servico.onrender.com,http://localhost:3000 \
  -e CACHE_TTL=5h \
  subtitle-delivery:local
```
