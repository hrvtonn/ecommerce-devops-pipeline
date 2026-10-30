# ecommerce-devops-pipeline

> Nome sugerido para este repositório no GitHub: **`ecommerce-devops-pipeline`**
> (deixa claro tanto o domínio da aplicação quanto o foco do desafio: esteira
> DevOps com CI/CD, IaC e observabilidade).

Solução do **Desafio Prático Final** (DevOps): API em Go + Frontend React,
containerizados, com pipeline CI/CD para Docker Hub, infraestrutura como código
via Terraform + LocalStack e observabilidade com Prometheus + Grafana.

## Arquitetura

```
                ┌──────────────┐        ┌──────────────┐
   usuário ───▶ │  frontend    │  /api ▶│     api      │──▶ Postgres
                │ React+Nginx  │        │   Go + chi   │──▶ Redis
                └──────────────┘        └──────┬───────┘
                                                │ /metrics
                                                ▼
                                         ┌──────────────┐      ┌──────────┐
                                         │  Prometheus  │◀────▶│ Grafana  │
                                         └──────────────┘      └──────────┘

  terraform/  ──▶ LocalStack (VPC, subnet, security group, EC2, bucket S3)
  .github/workflows/ ──▶ build & push das imagens para o Docker Hub
```

## Estrutura do repositório

```
.
├── cmd/api/                # entrypoint da API Go (serve | migrate | seed | healthcheck)
├── internal/               # API Go (handlers, services, repositories, validator, cache)
├── frontend/               # SPA React (Vite) que consome a API
├── Dockerfile              # imagem da API
├── frontend/Dockerfile     # imagem do frontend (build + Nginx)
├── docker-compose.yml      # stack completa para rodar tudo localmente
├── .github/workflows/      # pipeline CI/CD (GitHub Actions)
├── terraform/              # IaC para provisionar infra simulada no LocalStack
└── monitoring/
    ├── prometheus/         # prometheus.yml (scrape da API)
    └── grafana/            # datasource + dashboard provisionados automaticamente
```

## 1. Rodando tudo localmente (docker-compose)

Pré-requisitos: Docker e Docker Compose instalados.

```bash
cp .env.example .env   # ajuste se quiser, os defaults já funcionam
docker compose up -d --build
docker compose exec api /app/ecommerce-api seed   # popula categorias/produtos/clientes de exemplo
```

Serviços expostos:

| Serviço            | URL                              | Credenciais       |
|---------------------|-----------------------------------|--------------------|
| Frontend (React)    | http://localhost:8080            | -                  |
| API (Go)            | http://localhost:3000            | -                  |
| Swagger da API      | http://localhost:3000/api-docs   | -                  |
| Métricas Prometheus | http://localhost:3000/metrics    | -                  |
| Prometheus          | http://localhost:9090            | -                  |
| Grafana             | http://localhost:3001            | admin / admin      |
| LocalStack          | http://localhost:4566            | -                  |

No Grafana, o datasource do Prometheus e o dashboard **"E-Commerce API -
Observabilidade"** já vêm provisionados automaticamente (requisições/s, p95 de
latência, taxa de erros, CPU, memória, goroutines).

Para derrubar tudo: `docker compose down -v`.

## 2. Imagens publicadas no Docker Hub

As imagens já estão publicadas e públicas:

- **API**: [hrvton/ecommerce-api](https://hub.docker.com/r/hrvton/ecommerce-api)
- **Frontend**: [hrvton/ecommerce-frontend](https://hub.docker.com/r/hrvton/ecommerce-frontend)

```bash
docker pull hrvton/ecommerce-api:latest
docker pull hrvton/ecommerce-frontend:latest
```

Para publicar manualmente uma nova versão (fora da pipeline):

```bash
docker login
docker build -t hrvton/ecommerce-api:latest .
docker build -t hrvton/ecommerce-frontend:latest ./frontend
docker push hrvton/ecommerce-api:latest
docker push hrvton/ecommerce-frontend:latest
```

## 3. Pipeline CI/CD (GitHub Actions)

Arquivo: `.github/workflows/ci-cd.yml`. A cada `push` na branch `main`:

1. Roda `go vet` e os testes unitários da API (`go test -race ./...`).
2. Se os testes passarem, builda e publica no Docker Hub:
   - `hrvton/ecommerce-api:latest` e `:<sha do commit>`
   - `hrvton/ecommerce-frontend:latest` e `:<sha do commit>`

### Configuração necessária no GitHub

No repositório: **Settings → Secrets and variables → Actions → New repository
secret**:

| Secret               | Valor                                                    |
|----------------------|-----------------------------------------------------------|
| `DOCKERHUB_USERNAME` | seu usuário do Docker Hub                                  |
| `DOCKERHUB_TOKEN`    | um Access Token gerado em Docker Hub → Account Settings → Security |

## 4. Infraestrutura como código (Terraform + LocalStack)

O Terraform provisiona, dentro do LocalStack, os recursos que hospedariam os
containers em um ambiente AWS real:

- **VPC + subnet pública + internet gateway + route table** (rede)
- **Security Group** liberando as portas da API, frontend, Prometheus e Grafana
- **Instância EC2** (simulada) que representaria o host dos containers
- **Bucket S3** para artefatos de deploy

```bash
# 1. Suba o LocalStack (já incluso no docker-compose, ou isoladamente):
docker compose up -d localstack

# 2. Aplique a infraestrutura
cd terraform
terraform init
terraform apply -auto-approve

# 3. Veja os recursos criados
terraform output

# 4. Para destruir
terraform destroy -auto-approve
```

Isso já foi validado de ponta a ponta: `terraform apply` cria os 9 recursos no
LocalStack e `terraform destroy` os remove sem erros.

## 5. Observabilidade (Prometheus + Grafana)

- A API expõe métricas no padrão Prometheus em `GET /metrics`
  (`internal/metrics/metrics.go`), incluindo:
  - `http_requests_total` (contador por rota/método/status)
  - `http_request_duration_seconds` (histograma de latência)
  - métricas padrão do runtime Go e do processo (CPU, memória, goroutines, GC)
- `monitoring/prometheus/prometheus.yml` configura o scrape da API a cada 5s.
- `monitoring/grafana/provisioning/` provisiona automaticamente o datasource e
  o dashboard ao subir o container do Grafana — nada precisa ser configurado
  manualmente na interface.

## 6. Repositório no GitHub

Código-fonte: **https://github.com/hrvtonn/ecommerce-devops-pipeline**

```bash
git remote add origin https://github.com/hrvtonn/ecommerce-devops-pipeline.git
git push -u origin main
```

Os secrets `DOCKERHUB_USERNAME` e `DOCKERHUB_TOKEN` (seção 3) já estão
configurados no repositório e a pipeline já publicou as imagens com sucesso.

## 7. Roteiro para o Pitch Executivo (5 minutos)

| Tempo         | Foco                    | Pontos-chave a citar |
|---------------|-------------------------|------------------------|
| 00:00–01:00   | Problema e solução      | Deploys manuais e sem padronização geravam erro humano e falta de visibilidade. A esteira DevOps automatiza build/deploy e dá visibilidade em tempo real da saúde da aplicação. |
| 01:00–02:30   | Arquitetura e ferramentas | **Git/GitHub**: histórico e colaboração. **Docker/Docker Hub**: ambientes idênticos em qualquer máquina, imagens versionadas e distribuídas. **GitHub Actions**: build e publicação automáticos a cada push, elimina deploy manual. **Terraform + LocalStack**: infraestrutura versionada, reproduzível e testável sem custo de nuvem real. **Prometheus/Grafana**: métricas de CPU, requisições/s e latência em tempo real. |
| 02:30–04:30   | Demonstração prática    | 1) Alterar algo simples no código e dar `git push`. 2) Mostrar a Action rodando em Actions tab. 3) Mostrar a imagem nova no Docker Hub. 4) Rodar `terraform apply` no LocalStack. 5) Abrir o Grafana e gerar tráfego (`curl` em loop na API) mostrando o gráfico de requisições subir em tempo real. |
| 04:30–05:00   | Encerramento            | Ambiente reprodutível, testado automaticamente antes de cada deploy, infraestrutura auditável via código e anomalias detectáveis em segundos — não em dias. |

### Sugestão de comandos para a demonstração em vídeo/ao vivo

```bash
# Gerar tráfego para "animar" o dashboard do Grafana durante a demo
while true; do curl -s http://localhost:3000/api/v1/products > /dev/null; sleep 0.2; done
```

> O rate limit padrão é de 100 requisições por IP a cada 15 minutos; para a
> demo, aumente `RATE_LIMIT_MAX_REQUESTS` no serviço `api` do compose.
> `/health` e `/metrics` não entram no rate limit.

## Troubleshooting

- **`go mod download` ou `npm ci` falha com `EAI_AGAIN` durante o build da imagem**:
  algumas redes/sandboxes bloqueiam a resolução de DNS de dentro do container
  de build. Se isso ocorrer, builde manualmente com a rede do host:
  ```bash
  docker build --network=host -t <usuario>/ecommerce-api:latest .
  docker build --network=host -t <usuario>/ecommerce-frontend:latest ./frontend
  ```
  Em runners do GitHub Actions isso não costuma ser necessário.
- **Frontend reiniciando em loop com `host not found in upstream "api"`**:
  ocorre se o container `frontend` sobe antes do `api` existir na rede
  (ex.: primeiro `up` enquanto a imagem da API ainda falhava). O `nginx.conf`
  já usa `resolver 127.0.0.11` com resolução dinâmica; basta esperar o
  healthcheck do `api` ficar `healthy` ou rodar
  `docker compose restart frontend`.

## Testes

```bash
go test ./...                            # testes unitários (fakes em memória + miniredis, sem infra)
go test -race -cover ./...               # com detector de race e cobertura
```

Sem Go instalado localmente, rode via Docker:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.27-alpine go test ./...
```

## API em Go

- **Roteamento**: [chi](https://github.com/go-chi/chi); **PostgreSQL**: [pgx](https://github.com/jackc/pgx)
  (pool + transações); **Redis**: [go-redis](https://github.com/redis/go-redis);
  **métricas**: [client_golang](https://github.com/prometheus/client_golang); logs com `log/slog`
  (JSON em produção).
- Camadas: `httpapi` (rotas, middlewares, handlers) → `validator` → `service`
  (regras de negócio, cache, transações) → `repository` (SQL).
- Um único binário com subcomandos: `serve` (padrão, aplica as migrations ao
  subir), `migrate`, `seed` e `healthcheck` (usado pelo `HEALTHCHECK` da imagem,
  que é distroless e roda como usuário não-root).
- Variáveis de ambiente iguais às do `.env.example`; o ambiente é definido por
  `APP_ENV` (`development`, `production` ou `test`).
- Swagger UI em `/api-docs` e especificação em `/api-docs.json`
  (`internal/docs/openapi.json`).
