## Visao Geral

Este projeto expoe uma API Go enxuta que recebe URLs de legendas, valida com seguranca o conteudo baixado, armazena em cache a ultima URL de origem valida e retorna essa URL original para os clientes.

O comportamento atual e intencionalmente minimo: a API valida com seguranca o conteudo remoto da legenda, mas armazena e retorna apenas a URL original.

Principais capacidades:

- Aceitar URLs de legenda via `POST /subtitle`
- Retornar a ultima legenda valida via `GET /subtitle`
- Aceitar apenas extensoes de arquivo de legenda: `.srt`, `.vtt`, `.webvtt`
- Rejeitar conteudo malicioso de legenda
- Armazenar apenas metadados com expiracao temporaria de cache (sem servir o conteudo da legenda)
- Aplicar limites de tamanho de arquivo
- Aplicar limitacao de taxa
- Restringir acesso entre origens com CORS configuravel

## Requisitos

- Go `1.24.2`
- Docker, se voce quiser executar a imagem de container

## Estrutura Do Projeto

- `main.go`: ponto de entrada focado na inicializacao do processo
- `internal/infrastructure/config/config.go`: carregador de configuracao em runtime (`.env.dev`, `.env.prod` e variaveis de ambiente)
- `internal/domain/subtitle.go`: entidade de legenda e regras puras de dominio, como validacao e normalizacao
- `internal/service/subtitle_service.go`: orquestracao dos casos de uso de criacao e consulta de legenda
- `internal/controller/http_controller.go`: camada HTTP de traducao entre requisicoes e chamadas de servico
- `internal/httpapi/router.go`: registro das rotas HTTP
- `internal/httpapi/middleware.go`: middlewares HTTP de CORS e limitacao de taxa
- `internal/httpapi/api_integration_test.go`: testes de comportamento HTTP ponta a ponta, proximos da implementacao de transporte
- `internal/infrastructure/memory_store.go`: implementacao de cache em memoria com expiracao e invalidacao
- `internal/infrastructure/db/redis_store.go`: armazenamento de legendas com Redis e detalhes de persistencia relacionados ao banco
- `internal/infrastructure/fetcher.go`: implementacoes concretas de fetcher HTTP e de teste
- `internal/infrastructure/*_test.go`: testes proximos das implementacoes de infraestrutura
- `*_test.go`: testes HTTP e unitarios cobrindo os criterios de aceitacao
- `.env.dev`: configuracao padrao para desenvolvimento local
- `.env.prod`: modelo de producao com valores vazios

## Arquitetura

O projeto segue uma estrutura em camadas mais expressiva, inspirada em convencoes de project-layout para Go:

- `domain`: regras puras de negocio e modelo central de legenda
- `service`: casos de uso da aplicacao e orquestracao entre fetch e armazenamento
- `controller`: tratamento de entrada e saida especifico de HTTP
- `httpapi`: composicao de roteador e middlewares de transporte
- `infrastructure`: adaptadores como config, cache, db e fetchers
- `main`: montagem de dependencias e inicializacao do servidor

Isso mantem as regras de legenda fora da camada de transporte e facilita futuras mudancas de armazenamento ou transporte.

## Configuracao

A aplicacao carrega configuracao nesta ordem:

1. Variaveis de ambiente do processo
2. Valores de `.env.prod` quando `APP_ENV=production` e o valor nao estiver vazio
3. Valores de contingencia em `.env.dev`
4. Defaults internos quando necessario

Configuracoes disponiveis:

- `APP_ENV`: `development` ou `production`
- `APP_PORT`: porta do servidor HTTP
- `STORAGE_BACKEND`: `memory_cache` ou `redis`
- `REDIS_ADDR`: host e porta do Redis usados quando `STORAGE_BACKEND=redis`
- `REDIS_PASSWORD`: senha do Redis usada quando exigida pelo servidor
- `REDIS_DB`: numero do banco logico do Redis
- `REDIS_KEY_PREFIX`: prefixo aplicado as chaves de legenda no Redis
- `MAX_SUBTITLE_SIZE_BYTES`: tamanho maximo aceito para o payload da legenda
- `ALLOWED_ORIGINS`: lista de origens permitidas no CORS, separadas por virgulas
- `PROBE_ALLOWED_IPS`: lista de IPs permitidos para `/health`, separada por virgulas (vazio significa sem restricao de IP)
- `HEALTH_PROTECTION_ENABLED`: habilita ou desabilita protecao de IP para `/health`
- `CACHE_TTL`: duracao de expiracao do cache, por exemplo `10m`
- `RATE_LIMIT_BURST`: quantidade de requisicoes permitidas por janela por cliente
- `RATE_LIMIT_WINDOW`: duracao da janela de rate limit, por exemplo `1m`

Valores padrao de desenvolvimento ficam em `.env.dev`. Modelos de producao ficam em `.env.prod`.

## Execucao Local

```bash
go run .
```

A API inicia na porta definida em `APP_PORT`. Com a configuracao padrao de desenvolvimento, a URL base e `http://localhost:8080`.

## Execucao De Testes

```bash
go test ./... -count=1
```

A suite inclui um cenario operacional que valida uma legenda obtida de uma fonte HTTP local em processo e verifica armazenamento e recuperacao de URL sem dependencia de rede externa.

## Endpoints Da API

### GET /health

Retorna uma resposta simples de verificacao de vida.

Resposta de sucesso:

```json
{
	"status": "ok"
}
```

### POST /subtitle

Armazena uma legenda referenciada por URL.

Requisicao:

```json
{
	"url": "https://example.com/subtitles/movie.srt"
}
```

Resposta de sucesso:

```json
{
	"id": "c8f8cc0f5b1d4e33ae52e2a763c4e81d",
	"url": "https://example.com/subtitles/movie.srt"
}
```

Erros possiveis:

- `400 Bad Request`: corpo JSON invalido, extensao nao suportada ou conteudo malicioso de legenda
- `403 Forbidden`: origem nao permitida pelo CORS
- `413 Request Entity Too Large`: conteudo da legenda excede o tamanho maximo configurado
- `429 Too Many Requests`: rate limit excedido
- `502 Bad Gateway`: nao foi possivel buscar a legenda remota
- `500 Internal Server Error`: falha interna de persistencia ou geracao de identificador

### GET /subtitle

Retorna a URL da ultima legenda valida armazenada em cache.

Resposta de sucesso:

```json
{
	"id": "c8f8cc0f5b1d4e33ae52e2a763c4e81d",
	"url": "https://example.com/subtitles/movie.srt"
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

O acesso entre origens e controlado por `ALLOWED_ORIGINS`. A API:

- Aceita requisicoes preflight de origens permitidas
- Rejeita requisicoes de origens fora da allowlist
- Retorna `Access-Control-Allow-Origin` apenas para origens aceitas

## Limitacao De Taxa

A limitacao de taxa e aplicada por identificador de cliente derivado do endereco remoto da requisicao.

Notas de implementacao:

- Implementacao: `RateLimiter` em `internal/httpapi/rate_limiter.go`

Notas de comportamento:

- `/health` e excluida da limitacao de taxa de requisicoes.
- Requisicoes para `/subtitle` recebem limitacao de taxa por `RemoteAddr`.

Configuracao:

- `RATE_LIMIT_BURST`: numero maximo de requisicoes na janela configurada
- `RATE_LIMIT_WINDOW`: duracao da janela de reset

## Protecao Do Endpoint De Health

O endpoint `/health` suporta protecao opcional baseada em IP.

- Se `HEALTH_PROTECTION_ENABLED=false`, `/health` nao e restrito por IP.
- Se `HEALTH_PROTECTION_ENABLED=true` e `PROBE_ALLOWED_IPS` estiver vazio, `/health` nao e restrito por IP.
- Se `HEALTH_PROTECTION_ENABLED=true` e `PROBE_ALLOWED_IPS` estiver configurado, requisicoes de IPs nao listados recebem `403`.

## Docker

Build da imagem:

```bash
docker build -t subtitle-delivery:local .
```

Executar o container:

```bash
docker run --rm -p 8080:8080 \
	-e APP_ENV=development \
	-e ALLOWED_ORIGINS=http://localhost:3000 \
	subtitle-delivery:local
```

Executar o container com Redis como backend de armazenamento:

```bash
docker run --rm -p 8080:8080 \
	-e APP_ENV=production \
	-e STORAGE_BACKEND=redis \
	-e REDIS_ADDR=redis:6379 \
	-e REDIS_DB=0 \
	-e REDIS_KEY_PREFIX=subtitle-delivery \
	-e ALLOWED_ORIGINS=http://localhost:3000 \
	subtitle-delivery:local
```

## Trocando O Backend De Armazenamento

A fronteira de armazenamento e representada por `service.Store`. Para adicionar um novo backend:

1. Crie um novo tipo implementando `Save` e `Latest`
2. Mantenha o comportamento de expiracao e invalidacao explicito na implementacao do backend
3. Estenda `app.NewStorage` em `internal/app/setup.go` para selecionar o backend por `STORAGE_BACKEND`
4. Adicione testes para o comportamento do novo backend

## Como Contribuir

1. Adicione ou atualize testes primeiro
2. Mantenha as mudancas sem framework, a menos que uma biblioteca seja claramente justificada
3. Prefira arquivos focados por responsabilidade
4. Mantenha a documentacao alinhada com mudancas de configuracao e comportamento