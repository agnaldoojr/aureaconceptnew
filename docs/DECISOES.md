# Decisões técnicas (propostas na Fase 1; valem a partir da aprovação)

> Cada decisão tem o motivo e, quando existe, o que foi descartado. Qualquer desvio do pedido (stack, regras, visual) fica registrado aqui.
> Itens que dependem da sua resposta apontam para [DUVIDAS.md](DUVIDAS.md).

## Stack
| # | Decisão | Motivo |
|---|---|---|
| T-01 | **Go 1.22+ com `net/http` puro** (padrões de rota `"GET /produtos/{id}"` do próprio `ServeMux`), sem framework. O pedido permitia `chi`; só uso `chi` se algum recurso faltar | menos dependências e menos RAM; rotas do sistema são simples e a tabela rota → módulo → ação precisa ser explícita |
| T-02 | **Templates `html/template`** com funções próprias (dinheiro pt-BR, datas, permissões) e layout comum; templates e estáticos **embutidos** no binário (`embed`) | substitui o Thymeleaf; binário único |
| T-03 | **SQLite com `modernc.org/sqlite`** (sem CGO) | pedido; imagem final mínima e cross-compile fácil |
| T-04 | **Migrations com `goose` (SQL embutido)**, rodando no boot, versão registrada em tabela; o script destrutivo de limpeza **não existe** no projeto novo | pedido (golang-migrate/goose); goose é mais simples com `embed` |
| T-05 | **BCrypt** com `golang.org/x/crypto/bcrypt` (aceita os hashes `$2a$10$…` já existentes) | SEG-02 |
| T-06 | **Log** com `log/slog` (JSON em `prod`); configuração só por variáveis de ambiente (`internal/config`) | pedido |
| T-07 | **Node somente em `e2e/`** (Playwright, ferramentas de captura); fora da imagem de produção. PHP não é usado: nenhuma necessidade técnica real | pedido |

## Dinheiro, quantidades e arredondamento (sem `float`)
| # | Decisão | Motivo |
|---|---|---|
| T-08 | No banco, valores ficam como **inteiros escalados**: valores e preços ×10², quantidades ×10³, custo médio ×10⁴, fator ×10⁶, percentuais ×10² (colunas `INTEGER`). Soma e comparação no SQL são exatas | SQLite não tem `DECIMAL`; `REAL` perderia centavos |
| T-09 | Nas regras de negócio uso **`shopspring/decimal`** (Go puro) com os mesmos modos do Java: `HALF_UP` (`Round`), `ROUND_DOWN` (`Truncate`) e as escalas de cada operação (RN-EST-01/02, RN-FIN-02, RN-BAIXA-02, RN-COM, RN-META) | paridade exata de arredondamento; testes unitários comparam com valores calculados pelo Java |
| T-10 | O cálculo `docto_valor` segue o antigo (soma sem arredondar, arredonda uma vez) ⚠ D-05 | paridade |

## SQLite
| # | Decisão | Motivo |
|---|---|---|
| T-11 | `PRAGMA journal_mode=WAL`, `busy_timeout`, `foreign_keys=ON`, `synchronous=NORMAL`; **um `*sql.DB` de escrita com `MaxOpenConns(1)`** e outro só de leitura | pedido; evita `database is locked` |
| T-12 | **Comparação sem maiúscula e sem acento** (collation `Latin1_General_CI_AI` do SQL Server): registro uma *collation* `AUREA_CIAI` e uma função de normalização no driver; usada em busca (`LIKE`) e em **índices únicos** (login, sigla, nome de segmento) | INVENTARIO §8-1/2 |
| T-13 | Índice único parcial para `Meta_Venda` com vendedor nulo ⚠ D-07 | paridade da regra |
| T-14 | **Backup**: `VACUUM INTO` (consistente, sem parar o app) → `gzip` → rclone para o Drive; restore documentado e testado | pedido |
| T-15 | Transações: toda operação de documento/estoque/financeiro roda em **uma transação** de escrita (como o `@Transactional` do antigo) | RN-DOC-06 |

## Segurança e sessão
| # | Decisão | Motivo |
|---|---|---|
| T-16 | Sessão com **ID aleatório de 256 bits** em cookie `JSESSIONID` (mesmo nome do antigo), `HttpOnly`, `SameSite=Lax`, `Secure` por `COOKIE_SECURE`; armazenamento **persistido no SQLite** (decisão D-12: atualizar ou reiniciar não derruba os logins), uma sessão por usuário (a antiga expira); 8 h, ou 30 dias para `modoApp=1`; o limite de login continua em memória | SEG-05/06/07 |
| T-17 | **CSRF** com token por sessão: campo `_csrf` nos formulários e cabeçalho (meta `csrf-header`/`csrf-token` no Modo vendedor) idênticos ao Spring (`X-CSRF-TOKEN`), para o JS do antigo funcionar sem mudança | SEG-08 |
| T-18 | Limite de login em memória (IP+login e IP), mesmos números e janela | SEG-09 |
| T-19 | Tabela rota → módulo → ação **explícita e testada** (cada rota, cada ação), espelhando `PermissaoInterceptor` | SEG-11 |
| T-20 | Admin inicial por `ADMIN_SENHA_INICIAL` obrigatória, sem senha padrão (decisão D-01); a migração recusa concluir se o hash do admin ainda for o do `Admin@123` | SEG-03 |
| T-21 | **Direitos lidos do banco a cada requisição** (nome, situação ativa e as 4 ações por módulo), não congelados na sessão. Mudar uma permissão ou inativar o usuário vale na hora; no antigo valia só no próximo login. Custo: 2 consultas pequenas por tela, no pool de leitura | SEG-04, SEG-10 (melhoria de segurança, sem efeito visual) |
| T-22 | **Dependências de CDN servidas pelo próprio sistema** (decisão D-04): Font Awesome 6.5.0 (CSS + woff2), Chart.js 4.4.1 (UMD) e as fontes Inter 400–700 e Cormorant Garamond 400/600 (normal e itálico), subconjunto `latin`, vindos dos pacotes oficiais do npm (`@fortawesome/fontawesome-free`, `chart.js`, `@fontsource/*`) e guardados em `web/static/vendor/` com as licenças em `vendor/licencas/`. Sem requisição a terceiros: nenhum dado do usuário vai para Google/Cloudflare e o sistema funciona sem internet na rede da loja | UI-06 |
| T-23 | **Estáticos com hash no nome** (`/css/style-<32 hex do SHA-256>.css`) calculado na subida a partir do embed; o caminho sem hash também responde (como o Spring); URL com hash errado dá 404. Versionados: `max-age=31536000, immutable`; `vendor/` sem hash: 30 dias; o resto sem hash: `no-cache` + ETag (304) | INF-08 |
| T-24 | **Cookie de sessão `JSESSIONID`** com 256 bits aleatórios; no banco fica **só o SHA-256** do identificador (quem lê o arquivo do banco ou um backup não consegue usar as sessões). Sessão anônima só para o token CSRF do formulário de login (30 min), trocada por uma sessão nova no login | SEG-05, SEG-07, SEG-08 |
| T-25 | **IP do cliente atrás do Caddy** (`PROXY_CONFIAVEL`, padrão ligado em prod): vale o **último** valor de `X-Forwarded-For`, que o Caddy grava e não deixa o cliente forjar. Deixar ligado com o app exposto direto na internet permitiria forjar o IP e escapar do bloqueio de login (aviso no `.env.example`) | SEG-09 |
| T-26 | **Teste golden como prova de paridade**: `e2e/rodar-golden.sh <roteiro>` recria o antigo (banco descartável `AureaRef`) e o novo (arquivo temporário + `tools/demo`) e roda o MESMO roteiro nos dois; compara status, redirecionamento, título e o conteúdo da tela normalizado. É mais forte que testes escritos só pela leitura do código, porque o antigo é o oráculo | PARIDADE (tipo G) |

## Estrutura do repositório (proposta)
```
cmd/aurea/        servidor (+ subcomandos: backup, healthcheck)
cmd/migrar/       ferramenta SQL Server → SQLite (com relatório de divergências)
internal/config   variáveis de ambiente
internal/db       abertura, pragmas, collation, migrations (goose)
internal/dominio  regras puras e testáveis: estoque, financeiro, dre, agenda, comissão, metas, permissões, dinheiro
internal/web      handlers, middleware (sessão, CSRF, permissão, limite de login), views
internal/api      /api e /vendedor/api (contrato idêntico)
web/templates     html/template (portados dos 49 templates)
web/static        CSS/JS/imagens copiados SEM alteração (hash no nome na publicação)
migrations/       SQL versionado
e2e/              Playwright + ferramentas de captura (somente desenvolvimento)
docs/             INVENTARIO, PARIDADE, DUVIDAS, DECISOES, DESEMPENHO, referencia/ (telas e contrato do antigo)
deploy/           Caddyfile, backup.sh, restore.sh, atualizar.sh, compose
```

## Estratégia de testes (resumo; detalhe por item em PARIDADE.md)
1. **Unitários** das regras (`internal/dominio`), com casos tirados do Java (custo médio, conversão, parcelas, taxa, DRE, comissão, meta, permissões).
2. **Integração** (`httptest` + SQLite em arquivo temporário): cada rota web e `/api`, autenticação, permissões por módulo/direito, fluxos de ponta a ponta, migrations, backup/restore.
3. **Contrato da API** contra `docs/referencia/api/contrato.json` (capturado do sistema antigo): status, JSON e texto exato.
4. **Golden:** o mesmo `e2e/semear.mjs` roda no antigo e no novo e as telas/relatórios principais são comparados.
5. **E2E Playwright** por perfil, desktop e celular, acessibilidade básica e **comparação visual** com as capturas de `docs/referencia/`.
6. **Migração:** banco de referência SQL Server → SQLite com validação de contagens e somatórios.
