# Clinic Management API

Uma API HTTP em Go construída para gerenciar clínicas odontológicas e seus dentistas. O foco deste projeto foi criar um sistema robusto e fácil de manter, utilizando autenticação JWT, paginação eficiente (cursor-based), soft delete e uma stack de observabilidade pronta para produção.

## O que o sistema faz?

Basicamente, a API permite gerenciar clínicas (que obrigatoriamente precisam ter pelo menos uma conta bancária vinculada) e associar dentistas a elas. Os dentistas podem assumir papéis específicos dentro de cada clínica, como administradores ou representantes legais.

Para garantir a integridade dos dados, implementei soft delete (remoção lógica, preservando o histórico) e tratei a concorrência em operações críticas usando locks pessimistas e retries semânticos.

## Como rodar o projeto

Nos exemplos, você vera que sempre adiciono o `Podman` por uma preferência pessoal. Porém, todo projeto deve funcionar com o Docker normalmente.
Você vai precisar do [Podman](https://podman.io/) (ou Docker) e do [Bruno](https://www.usebruno.com/) se quiser testar as rotas de forma visual. O arquivo `.env` já está na raiz com as variáveis pré-configuradas para o ambiente local.

Para subir a stack inteira, basta rodar:

```bash
cd capim-test
podman compose up --build
```

Isso vai levantar os seguintes serviços:

- **API**: rodando na porta `8081` (Health check disponível em `http://localhost:8081/api/v1/health`)
- **Grafana**: rodando na porta `3000` (usuário: `admin` / senha: `admin`)

Para derrubar a stack, use `podman compose down`.

## Testando a API

Deixei uma coleção do Bruno pronta na pasta `tests/bruno/clinic-management-api`. É o jeito mais fácil de explorar os endpoints.

Basta abrir o Bruno, importar a pasta e executar os requests. A coleção já está organizada numa sequência lógica: primeiro você faz o Login (que salva o token JWT automaticamente nas variáveis de ambiente do Bruno), e depois pode testar o CRUD de clínicas e o vínculo de dentistas sem se preocupar em ficar copiando e colando o token a cada requisição.

## Stack e Arquitetura

A aplicação foi desenhada em camadas bem definidas para separar as responsabilidades e facilitar a criação de testes.

- **Web/HTTP**: Gin
- **Banco de Dados**: PostgreSQL com queries type-safe geradas pelo `sqlc`. Preferi o sqlc a um ORM tradicional para ter mais controle sobre o SQL e uma performance mais previsível.
- **IDs**: UUIDv7. Eles são ótimos porque mantêm a ordenação temporal no banco de dados, ajudando na performance de índices e paginação.
- **Erros**: Seguem a RFC 9457 (Problem Details), padronizando as respostas de erro para quem consome a API.
- **Observabilidade**: OpenTelemetry integrado com a stack LGTM (Grafana, Loki, Tempo, Mimir).

```text
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   HTTP      │────▶│  Service    │────▶│ Repository  │
│  (Gin)      │     │  (Regras)   │     │   (sqlc)    │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
  RFC 9457            Validações          PostgreSQL
  OpenTelemetry       Transações          Soft Delete
```

## Principais Endpoints

A API é protegida por JWT (Bearer token), com exceção das rotas de login e health check.

**Autenticação & Saúde**

- `POST /api/v1/auth/login` (Público)
- `GET /api/v1/health` (Público)

**Clínicas**

- `GET /api/v1/clinics` (Listagem com paginação via cursor)
- `POST /api/v1/clinics` (Criação)
- `GET /api/v1/clinics/:id` (Detalhes da clínica, incluindo contas bancárias)
- `PATCH /api/v1/clinics/:id` (Atualização)
- `DELETE /api/v1/clinics/:id` (Soft delete)

**Dentistas**

- `POST /api/v1/clinics/:id/dentists` (Vincular ou criar dentista)
- `GET /api/v1/clinics/:id/dentists` (Listar dentistas de uma clínica)
- `PATCH /api/v1/clinics/:id/dentists/:dentist_id` (Atualizar papéis do dentista na clínica)
- `DELETE /api/v1/clinics/:id/dentists/:dentist_id` (Desvincular dentista)
- `PATCH /api/v1/dentists/:id` (Atualizar dados pessoais do dentista)
- `DELETE /api/v1/dentists/:id` (Deletar dentista)

## Contratos e Paginação

A paginação utiliza cursores em vez de offsets para garantir uma performance constante, mesmo quando a base de dados cresce. Você pode passar os parâmetros `limit` (padrão 20, máximo 100) e `cursor` (o UUIDv7 da última página) na query string. A resposta inclui headers úteis como `X-Next-Cursor` e `Link` para facilitar a navegação para a próxima página.

Em caso de erro, a API retorna um JSON detalhado:

```json
{
  "type": "https://capim.test/problems/validation-error",
  "title": "Validation Error",
  "status": 400,
  "detail": "invalid CNPJ",
  "instance": "/api/v1/clinics",
  "request_id": "7dc1e50c-8e6b-42f2-b9db-589f0df723dc"
}
```

## Testes e Observabilidade

Para rodar os testes unitários em Go:

```bash
go test ./...
```

Se quiser rodar os testes de integração com Hurl (opcional):

```bash
just test-hurl-docker
```

Com a stack rodando, recomendo muito abrir o Grafana (`http://localhost:3000`) e dar uma olhada no dashboard "Capim API - Observability". Lá você vai encontrar os traces das requisições HTTP, métricas de latência, throughput e logs estruturados.

## Decisões de Projeto

Alguns pontos que valem a pena destacar sobre a construção da API:

1. **Arquitetura simples e direta**: Optei por uma separação clássica. Os handlers HTTP só lidam com o contrato da web (requests/responses), enquanto as regras de negócio ficam isoladas nos services. O uso do `sqlc` na camada de dados traz segurança de tipagem sem a "mágica" e a complexidade de um ORM.
2. **Foco na experiência de quem consome a API**: Implementar a RFC 9457 para erros e usar paginação baseada em cursor mostra uma preocupação com a previsibilidade do sistema. Além disso, a observabilidade foi pensada desde o dia zero, o que facilita muito o debug em produção.
3. **Lidando com concorrência**: Em operações sensíveis, como garantir que uma clínica sempre tenha pelo menos uma conta bancária ativa, utilizei locks pessimistas (`SELECT FOR UPDATE`) no banco de dados, aliados a retries semânticos para não prejudicar a experiência do usuário com erros de concorrência.

**O que eu faria com mais tempo?**

- Implementaria um controle de acesso (ACL). No geral, toda a parte de segurança está bem básica e precisaria ser refinada.
- Aumentaria o cenário de teste integrado e criaria uma pipeline para eles.
- Adicionaria documententação dos endpoint com Openapi.
- Os dashboards do grafana poderão ser muito mais refinados e trazer muito mais informação relevante.

**Uso de IA**

O uso de IA sempre me ajuda na inicialização do projeto, comigo detalhando o que eu quero dele e a IA trazendo para mim uma base. Com a casca do projeto criado, eu gosto de utilizar a nível de funções e no máximo de arquivo. Assim eu consigo ter controle maior e ainda assim conseguir revisar o que está sendo gerado por ela. Qualquer coisa além disso, eu sinto que eu gasto mais tempo revisando do que teria gasto escrevendo.
Também utilizei IA para a geração das collections do Bruno e do Hurl.
Ao iniciar os testes e percebi que estava tendo alguns problemas de concorrência no momento da criação de dentista. E nesse momento eu decidi questionar os melhores approutes para esse caso específico. Ele me trouxe algumas soluções e acabei escolhendo de utilizar lock pessimista, gosto de seguir esse fluxo porque a IA consegue condensar muito bem pontos negativos e positivos dos diversos approaches que seriam possíveis. E eu consigo focar mais na solução.
