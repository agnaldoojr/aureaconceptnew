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

## Pronto e testado (fatia 1: alicerce)
| Pacote | O que tem | Testes (25 passando) |
|---|---|---|
| `internal/dominio` | Regras puras com decimais exatos (`shopspring/decimal`): quantidade e valor na unidade base, **custo médio ponderado**, total do item e do documento, **parcelas** (ROUND_DOWN + resto), taxa de cartão, comissão, preço sugerido, valor movimentado, status do título, progresso da meta | **1.212 casos gerados pelo Java do sistema antigo** (`tools/golden/Golden.java` → `testdata/golden/dinheiro.csv`) conferem com o Go + casos de borda (fator 0, arredondamento do documento D-05, exemplo da caixa do README, barra da meta; o dashboard de referência dá 19 % e falta R$ 8.024,50) |
| `internal/db` | Abertura do SQLite (`modernc.org/sqlite`, sem CGO): WAL, `busy_timeout`, `foreign_keys`, **uma conexão de escrita**, pool de leitura somente leitura, collation `AUREA_CIAI` (sem maiúscula e sem acento, como o `Latin1_General_CI_AI`) e função `aurea_norm`; migrations goose embutidas rodando na abertura | 25 tabelas criadas, migration idempotente, dados iniciais iguais ao antigo (2 segmentos, 5 unidades, 9 tipos, 3 contas, 12 categorias, 9 formas com taxa/parcelas), pragmas, escrita bloqueada no pool de leitura, FK barra órfãos, **login único sem diferenciar `João`/`JOAO`/`joão  `**, busca sem acento, 13 restrições do esquema (CHECKs) e a regra D-07 da meta |
| `migrations/00001_baseline.sql` | Estado final do banco antigo (schema.sql + 02 a 13) em SQLite: valores como inteiros escalados (centavos, milésimos, décimos de milésimo, milionésimos) | coberto pelos testes de `internal/db` |
| `internal/auth` | BCrypt fator 10 | **hashes reais gerados pelo Java** (admin e 4 usuários do banco de referência) continuam válidos; hash novo é `$2a$10$`/`$2b$10$`; senha errada ou vazia nunca confere |
| `internal/permissoes` | 21 módulos × 4 direitos, rótulos, `admin` sempre com tudo, 4 perfis prontos, **mapa rota → módulo → ação** idêntico ao `PermissaoInterceptor`, mensagens de negação | ~85 rotas reais conferidas (incluindo `salvar` com/sem código, agenda bloqueios/horários, financeiro, `/vendedor`, BI e lucratividade exigindo `VENDAS_TODOS`, barra final como o `split` do Java) |

Itens da matriz atualizados: INF-02, INF-03 (`feito`); SEG-02, SEG-10, SEG-11, SEG-13 (`feito`, faltam integração/migração); e a regra pura de DOC-04, DOC-07, DOC-08, FIN-02, FIN-05, FIN-07, CAD-06, GES-03, SAL-02 (continuam `pendente` até a tela/rota existir e os testes I/G passarem).
Nenhum item está `testado` ainda: falta o servidor.

## Próximas fatias (ordem proposta)
1. **Configuração por variáveis de ambiente** (`internal/config`) e admin inicial por `ADMIN_SENHA_INICIAL` (SEG-03, INF-04/05).
2. **Servidor HTTP**: sessão única persistida no SQLite (D-12), CSRF, limite de login, middleware de permissão (usa `internal/permissoes`), layout base e estáticos copiados do antigo com hash no nome (INF-08, UI-01/02/06).
3. **Cadastros** (produtos com unidades e serviços, unidades, pessoas, usuários, permissões) + testes de integração e E2E.
4. **Documentos + estoque + financeiro** (o núcleo transacional) com os testes *golden* (`e2e/semear.mjs` nos dois sistemas).
5. **API `/api` e `/vendedor/api`** contra `docs/referencia/api/contrato.json`.
6. **Salão** (agenda, comissões), **metas, dashboard, BI, relatórios**.
7. **Backup/restore para o Google Drive**, Docker/Caddy, `cmd/migrar`, medição de RAM.

## Pendências de você
- **D-08**: como usar o rea.tools na seção ENGENHARIA e o que faltou no fim do pedido ("passo a passo de…").
- Mais adiante (**D-10**): um backup `.bak` do banco da VPS para ensaiar a migração.
