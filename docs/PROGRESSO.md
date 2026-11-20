# Progresso da Fase 2

> Atualizado em 20/11/2026. A fonte da verdade do status por funcionalidade é [PARIDADE.md](PARIDADE.md). Aqui fica o resumo do que existe,
> do que foi testado (e como) e do que vem a seguir.

## Como rodar os testes
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path
go vet ./...
go test ./... -count=1
```
Todos os testes atuais são autônomos (SQLite em pasta temporária; nada de rede nem de SQL Server). Os arquivos *golden* vêm do sistema antigo e estão versionados em `testdata/golden/`.

## Pronto e testado (fatias 1 e 2)

### Fatia 1: alicerce
| Pacote | O que tem | Testes |
|---|---|---|
| `internal/dominio` | Regras puras com decimais exatos (`shopspring/decimal`): quantidade e valor na unidade base, **custo médio ponderado**, total do item e do documento, **parcelas** (ROUND_DOWN + resto), taxa de cartão, comissão, preço sugerido, valor movimentado, status do título, progresso da meta | **1.212 casos gerados pelo Java do sistema antigo** (`tools/golden/Golden.java` → `testdata/golden/dinheiro.csv`) conferem com o Go + casos de borda (fator 0, arredondamento do documento D-05, exemplo da caixa do README, barra da meta; o dashboard de referência dá 19 % e falta R$ 8.024,50) |
| `internal/db` | Abertura do SQLite (`modernc.org/sqlite`, sem CGO): WAL, `busy_timeout`, `foreign_keys`, **uma conexão de escrita**, pool de leitura somente leitura, collation `AUREA_CIAI` (sem maiúscula e sem acento, como o `Latin1_General_CI_AI`) e função `aurea_norm`; migrations goose embutidas rodando na abertura | 25 tabelas criadas, migration idempotente, dados iniciais iguais ao antigo (2 segmentos, 5 unidades, 9 tipos, 3 contas, 12 categorias, 9 formas com taxa/parcelas), pragmas, escrita bloqueada no pool de leitura, FK barra órfãos, **login único sem diferenciar `João`/`JOAO`/`joão  `**, busca sem acento, 13 restrições do esquema (CHECKs) e a regra D-07 da meta |
| `migrations/00001_baseline.sql` | Estado final do banco antigo (schema.sql + 02 a 13) em SQLite: valores como inteiros escalados (centavos, milésimos, décimos de milésimo, milionésimos) | coberto pelos testes de `internal/db` |
| `internal/auth` | BCrypt fator 10 | **hashes reais gerados pelo Java** (admin e 4 usuários do banco de referência) continuam válidos; hash novo é `$2a$10$`/`$2b$10$`; senha errada ou vazia nunca confere |
| `internal/permissoes` | 21 módulos × 4 direitos, rótulos, `admin` sempre com tudo, 4 perfis prontos, **mapa rota → módulo → ação** idêntico ao `PermissaoInterceptor`, mensagens de negação | ~85 rotas reais conferidas (incluindo `salvar` com/sem código, agenda bloqueios/horários, financeiro, `/vendedor`, BI e lucratividade exigindo `VENDAS_TODOS`, barra final como o `split` do Java) |

Itens da matriz atualizados na fatia 1: INF-02, INF-03 (`feito`); SEG-02, SEG-10, SEG-11, SEG-13 (`feito`, faltam integração/migração); e a regra pura de DOC-04, DOC-07, DOC-08, FIN-02, FIN-05, FIN-07, CAD-06, GES-03, SAL-02 (continuam `pendente` até a tela/rota existir e os testes I/G passarem).
(Na fatia 3 os primeiros itens passaram a `testado`.)

### Fatia 2: configuração e admin inicial
| Pacote | O que tem | Testes |
|---|---|---|
| `internal/config` | Configuração só por variáveis de ambiente (`APP_ENV`, `PORT`, `BIND`, `DB_PATH`, `DOMINIO`, `COOKIE_SECURE`, sessões, limites de login, `FUSO`, logs, `ADMIN_SENHA_INICIAL`, backup). **Reúne todos os erros de uma vez**; em `prod` exige `DOMINIO` e liga cookie seguro e log JSON por padrão; fuso `America/Sao_Paulo` **embutido no binário** (`time/tzdata`); senha inicial validada (10+ caracteres, maiúscula, minúscula, número, fora de uma lista de senhas conhecidas, até 72 bytes por causa do BCrypt) | padrões de dev, fuso (12:00 UTC = 09:00 em São Paulo), prod sem domínio recusa, `COOKIE_SECURE=false` explícito vale, 7 erros reunidos numa só mensagem, valores personalizados, 6 senhas ruins recusadas |
| `internal/bootstrap` | `GarantirAdmin`: sem `admin`, cria (nome Administrador, hash BCrypt, 21 módulos com tudo marcado) a partir de `ADMIN_SENHA_INICIAL`; com `admin`, **nunca altera a senha** e dispensa a variável; recusa o hash do `Admin@123` do sistema antigo | SQLite real: cria e o login casa sem diferenciar maiúscula; sem senha recusa e não cria nada; senha fraca ou `Admin@123` recusadas; segundo boot não troca a senha; hash do `schema.sql` antigo recusado |
| `.env.example` | Modelo comentado de todas as variáveis | — |

Itens da matriz: INF-04, INF-07 e SEG-03 `feito`; INF-05 segue `pendente` (a recusa de configuração insegura passa nos testes; faltam cookie, logs, shutdown e compressão, que dependem do servidor).

### Fatia 3: servidor HTTP (login, sessão, CSRF, permissões, layout e estáticos)
| Pacote / pasta | O que tem | Testes |
|---|---|---|
| `internal/web` (`servidor.go`) | Cadeia: cabeçalhos de segurança → estáticos e `/healthz` → `/login` e `/logout` → **sessão → login obrigatório → CSRF → permissão (`permissoes.Decidir`) → tela**. Login por formulário (`login`, `senha`, `modoApp`) com as mesmas mensagens do antigo, tempo de BCrypt gasto mesmo se o login não existe, **id de sessão novo a cada login**; uma sessão por usuário (a antiga vai a `/login?expired=true`); inativo não entra; negação volta ao `/dashboard` com mensagem de uma só leitura; `/bi` e lucratividade pedem `VENDAS_TODOS` | ver os 32 testes de integração do pacote (colunas ao lado) |
| `internal/web/sessao.go` + `migrations/00002_sessoes.sql` | Sessões **no SQLite** (D-12), só o SHA-256 do id no banco; anônima de 30 min só para o token CSRF; renovação por inatividade; limpeza periódica | expira por inatividade (8 h) e renova; app instalado 30 dias com `Max-Age`; sobrevive a "reiniciar" (segundo `Servidor` no mesmo banco); limpeza apaga só as vencidas |
| `internal/web/throttle.go` | Limite de login: 5 por IP+login, 30 por IP, bloqueio de 15 min, zera no sucesso, falha antiga recomeça a conta; IP do `X-Forwarded-For` atrás do Caddy | unitário com relógio manual + integração (5 erros bloqueiam até com a senha certa; 30 erros bloqueiam todos os logins do IP; IP forjado não escapa) |
| `internal/web/estaticos.go` | `/css`, `/js`, `/img`, `/vendor` com hash no nome, 365 dias, ETag/304; `/favicon.ico` | toda URL das telas tem hash e responde 200/304; hash errado 404; tipos corretos (webp, woff2, json) |
| `web/templates` (`layout/base.html`, `login.html`, `dashboard.html`) | Menu lateral por permissão (grupos Loja/Salão/Gerência), topo, rodapé e alertas, portados do Thymeleaf; **o dashboard ainda é provisório** | menu do admin completo, menu enxuto de quem só tem 2 módulos, mudança de permissão vale na hora |
| `web/static` | CSS, JS, imagens e PWA do antigo **sem alteração**; `vendor/`: Font Awesome 6.5.0, Chart.js 4.4.1, Inter e Cormorant (D-04, com as licenças) | no navegador de verdade: login e dashboard com fonte, ícones e menu certos, **nenhuma requisição externa**, console sem erros |
| `cmd/aurea` | `aurea` (config → banco → admin → servidor, desligamento gracioso, limpeza de sessões a cada 15 min) e `aurea healthcheck` | rodado de verdade (login e dashboard no Edge); **RAM medida: 68 MB de memória residente no Windows** com login feito (o Java do antigo fica em ~330–380 MB); número de Linux/Docker só na hora de medir o contêiner |

Itens da matriz: `testado` INF-08, SEG-04, SEG-05, SEG-06, SEG-07, SEG-10, SEG-11; `feito` INF-05, INF-06, SEG-08, SEG-09, SEG-12, UI-01, UI-02, UI-03, UI-06 (faltam E2E, o 429 da API e o `/api`).
**Não testado ainda:** E2E Playwright e comparação visual automática (só vi o login no navegador e ele bate com a captura de referência), `/api` e `/vendedor/api`, celular real, Docker.
**Correção:** na fatia 2 eu tinha criado uma "janela de 30 min" no limite de login; o antigo não tem isso (são 5 por IP+login, **30 por IP** e 15 min de bloqueio). Corrigido: `LOGIN_TENTATIVAS_IP=30`, sem `LOGIN_JANELA_MIN`.

### Fatia 4: cadastros de Unidades de medida e Pessoas
| Pacote / pasta | O que tem | Testes |
|---|---|---|
| `internal/cadastro` | Regras de `UnidadeMedidaService` e `PessoaService` do antigo, com as **mesmas mensagens**: sigla única em maiúsculas (≤ 10, sem espaço), descrição ≤ 60, sigla em uso não troca e unidade em uso não exclui; pessoa com nome ≤ 150, CPF/CNPJ único **só pelos dígitos** (quem já tinha o número pode ser editado), telefone repetido só avisa, nascimento não futuro, segmento inativo só se a pessoa já o tinha, exclusão em transação (documento vinculado barra e nada se perde) | 13 testes de integração + 2 unitários em `internal/web/cadastros_test.go` (SQLite real, telas e permissões) |
| `internal/web` (`unidades.go`, `pessoas.go`, `cadastros.go`) | Rotas `/unidades` e `/pessoas` (lista, novo, editar, salvar, status, excluir), ligação de formulário como o Spring (caixa de marcação com o marcador `_campo`, 400 para número ruim, `redirect` + mensagem de uma só leitura) | idem |
| `web/templates/unidades`, `web/templates/pessoas` | 4 telas portadas do Thymeleaf | **golden contra o antigo** (abaixo) |
| `e2e/comparar-cadastros.mjs` + `e2e/rodar-golden.sh` + `e2e/lib/html.mjs` | **Roteiro de 83 passos rodado no sistema antigo e no novo**, ambos recriados do zero: compara status HTTP, redirecionamento, título e a **área de conteúdo da tela inteira** (HTML normalizado: atributos ordenados, entidades decodificadas, espaços e comentários ignorados; 100 a 530 linhas por tela) | **83 passos, 0 divergências** |
| `tools/demo` | Carrega no banco novo os dados de exemplo que o `schema.sql` antigo cria (8 produtos, 8 pessoas, estoque, uma compra), **só para o golden** | usado pelo `rodar-golden.sh` |

O golden achou, de saída, duas coisas que os testes escritos por mim não achariam: o banco antigo **já nasce com dados de exemplo** (D-19) e a entidade `&ecirc;` do Thymeleaf. Também confirmou comportamentos sutis do antigo: o menu não destaca "Pessoas" (D-18), excluir pessoa inexistente diz "Pessoa excluída!", editar com código que não existe **insere** uma pessoa nova, e erro de validação do nome/nascimento volta ao formulário com o digitado enquanto erro de CPF repetido volta à lista com a mensagem.

Itens da matriz: `feito` CAD-08, CAD-09, CAD-10, CAD-11 (falta o E2E Playwright e o C da API); CAD-15 segue `pendente` (só Pessoas usa segmento por enquanto).
**Não testado ainda:** E2E Playwright, pixel a pixel (a tela é o mesmo HTML com o mesmo CSS, mas não comparei imagens), celular real.

## Próximas fatias (ordem proposta)
1. ~~Configuração por variáveis de ambiente e admin inicial~~ (feito na fatia 2).
2. ~~Servidor HTTP: sessão, CSRF, limite de login, permissões, layout e estáticos~~ (feito na fatia 3; falta o conteúdo do dashboard).
3. **Cadastros**: ~~unidades e pessoas~~ (fatia 4); faltam **produtos** (unidades extras, serviços, ativar/inativar, excluir D-03), **usuários** e **permissões** (perfis prontos).
4. **Documentos + estoque + financeiro** (o núcleo transacional) com os testes *golden* (`e2e/semear.mjs` nos dois sistemas).
5. **API `/api` e `/vendedor/api`** contra `docs/referencia/api/contrato.json`.
6. **Salão** (agenda, comissões), **metas, dashboard, BI, relatórios**.
7. **Backup/restore para o Google Drive**, Docker/Caddy, `cmd/migrar`, medição de RAM.

## Pendências de você
- **D-08**: como usar o rea.tools na seção ENGENHARIA e o que faltou no fim do pedido ("passo a passo de…").
- Mais adiante (**D-10**): um backup `.bak` do banco da VPS para ensaiar a migração.
