# To-Do List API

API REST para gerenciamento de tarefas, desenvolvida em Go com persistencia em MongoDB.

O projeto permite criar, listar, consultar, atualizar e remover tarefas, com validacoes de negocio, paginacao, filtros e documentacao OpenAPI acessivel via Swagger.

## Funcionalidades

- Criacao de tarefas com titulo, descricao, prioridade e data limite.
- Listagem paginada de tarefas.
- Filtro de tarefas por status e prioridade.
- Busca de tarefa por ID.
- Atualizacao parcial de tarefa existente via `PUT` ou `PATCH`.
- Remocao de tarefa por ID.
- Health check da aplicacao em `/health`.
- Documentacao interativa em `/swagger`.
- Logs estruturados com `X-Request-ID` em todas as requisicoes.
- Suite de testes unitarios, integracao HTTP e E2E com MongoDB.

## Regras de negocio

- O campo `title` e obrigatorio e deve ter entre 3 e 100 caracteres.
- O campo `priority` e obrigatorio e aceita apenas: `low`, `medium`, `high`.
- O campo `due_date` e obrigatorio, deve estar no formato `YYYY-MM-DD` e nao pode estar no passado.
- Toda tarefa e criada com status inicial `pending`.
- Os status permitidos sao: `pending`, `in_progress`, `completed`, `cancelled`.
- Uma tarefa com status `completed` nao pode ser atualizada.
- O ID da tarefa deve ser um UUID valido.
- A atualizacao exige pelo menos um campo no payload.
- A listagem usa paginacao com valores padrao `page=1` e `page_size=10`.
- O valor maximo de `page_size` e `100`.
- A listagem e ordenada pelas tarefas mais recentes primeiro.

## Endpoints

| Metodo | Rota | Descricao |
| --- | --- | --- |
| `GET` | `/health` | Verifica se a API esta online |
| `GET` | `/swagger` | Interface Swagger UI |
| `GET` | `/swagger/openapi.json` | Especificacao OpenAPI |
| `GET` | `/tasks` | Lista tarefas com filtros e paginacao |
| `POST` | `/tasks` | Cria uma nova tarefa |
| `GET` | `/tasks/{id}` | Busca uma tarefa por ID |
| `PUT` | `/tasks/{id}` | Atualiza uma tarefa com payload parcial |
| `PATCH` | `/tasks/{id}` | Atualiza parcialmente uma tarefa |
| `DELETE` | `/tasks/{id}` | Remove uma tarefa |

### Parametros da listagem

- `status`: filtra por status.
- `priority`: filtra por prioridade.
- `page`: numero da pagina.
- `page_size`: quantidade de itens por pagina.

Exemplo:

```bash
curl "http://localhost:8080/tasks?status=pending&priority=high&page=1&page_size=10"
```

## Exemplo de payload

### Criar tarefa

```json
{
  "title": "Estudar Go",
  "description": "Revisar camadas controller, service e repository",
  "priority": "high",
  "due_date": "2030-01-15"
}
```

### Atualizar tarefa (`PUT` ou `PATCH`)

```json
{
  "status": "in_progress",
  "priority": "medium"
}
```

## Tecnologias

- Go `1.25`
- MongoDB
- `net/http` da biblioteca padrao
- `slog` para logging
- Driver oficial MongoDB para Go

## Como rodar o projeto

### Pre-requisitos

- Go `1.25` ou superior
- Docker e Docker Compose, ou uma instancia local do MongoDB

### 1. Subir o MongoDB com Docker Compose

```bash
docker compose up -d mongo
```

O `docker-compose.yml` do projeto sobe um MongoDB local na porta `27017`.

### 2. Configurar variaveis de ambiente

Se nada for configurado, o projeto usa os valores padrao abaixo:

| Variavel | Descricao | Valor padrao |
| --- | --- | --- |
| `APP_PORT` | Porta HTTP da aplicacao | `8080` |
| `MONGO_URI` | URL de conexao com MongoDB | `mongodb://localhost:27017` |
| `MONGO_DB` | Nome do banco | `appdb` |
| `MONGO_COLLECTION` | Nome da colecao de tarefas | `tasks` |
| `LOG_LEVEL` | Nivel de log | `info` |
| `LOG_FORMAT` | Formato do log (`text` ou `json`) | `text` |

Exemplo:

```bash
export APP_PORT=8080
export MONGO_URI=mongodb://localhost:27017
export MONGO_DB=appdb
export MONGO_COLLECTION=tasks
export LOG_LEVEL=info
export LOG_FORMAT=text
```

### 3. Rodar a aplicacao

```bash
go run .
```

A API ficara disponivel em:

- `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger`
- OpenAPI JSON: `http://localhost:8080/swagger/openapi.json`

### 4. Validar se a API subiu

```bash
curl http://localhost:8080/health
```

Resposta esperada:

```json
{
  "status": "ok"
}
```

## Estrutura do projeto

```text
.
|-- main.go
|-- docker-compose.yml
|-- internal
    |-- config
    |-- controllers
    |-- database
    |-- docs
    |-- handlers
    |-- logger
    |-- models
    |-- repositories
    |-- server
    |   `-- test
    |-- services
    |   `-- test
    `-- utils
`-- test
    `-- e2e
```

Resumo das camadas:

- `controllers`: trata HTTP, valida entrada e monta respostas.
- `services`: concentra regras de negocio.
- `repositories`: faz acesso ao MongoDB.
- `docs`: expoe Swagger UI e OpenAPI.
- `server`: configura rotas e middleware.

Arquivos de teste:

- `internal/services/test/task_service_test.go`: testes unitarios das regras de negocio.
- `internal/server/test/router_integration_test.go`: testes de integracao HTTP com repositorio em memoria.
- `test/e2e/task_api_e2e_test.go`: testes E2E com MongoDB real e servidor HTTP completo.

## Testes

### O que esta coberto

- Regras de negocio de criacao, atualizacao e exclusao.
- Validacao de titulo, prioridade, status e `due_date`.
- Bloqueio de edicao de tarefas com status `completed`.
- Filtros e paginacao na listagem.
- Fluxo completo dos endpoints com persistencia real no MongoDB.

### Rodar testes unitarios e de integracao

```bash
go test ./...
```

### Rodar testes E2E

Os testes E2E usam a build tag `e2e` e exigem MongoDB disponivel.

1. Suba o MongoDB:

```bash
docker compose up -d mongo
```

2. Rode os testes:

```bash
MONGO_URI=mongodb://127.0.0.1:27017 go test -tags=e2e ./test/e2e -v
```

### CI

A pipeline em `.github/workflows/ci.yml` executa:

- verificacao de formatacao com `gofmt`
- `go test ./... -v`
- testes E2E com `go test -tags=e2e ./test/e2e -v`
- `go build ./...`

## Verificacao

O projeto foi validado com:

```bash
go test ./...
```

E para a suite E2E:

```bash
go test -tags=e2e ./test/e2e -run '^$'
```

No estado atual, os testes automatizados incluem unitarios, integracao HTTP e E2E.
