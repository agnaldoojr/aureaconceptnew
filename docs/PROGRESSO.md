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

### Fatia 5: Produtos (com unidades extras, serviços, status e exclusão corrigida)
| Pacote / pasta | O que tem | Testes |
|---|---|---|
| `internal/cadastro/produto.go` | Regras do `ProdutoService` e `ProdutoUnidadeService` com as mesmas mensagens e a **mesma ordem de verificações**: validação de campos (anotações do modelo), troca de base com extras, unidade cadastrada/ativa (o produto mantém a sua mesmo inativa), serviço só com saldo zero, produto zera comissão e duração, linha de estoque criada/removida, unidades extras (repetida, igual à base, fator, preço), tudo numa transação; status (ativar exige preço ≥ 0,01); **exclusão corrigida (D-03)**; `queimarCodigo` | 13 testes de integração em `internal/web/produtos_test.go` |
| `internal/web` (`produtos.go`, `formato.go`) | Rotas `/produtos` (lista, novo, editar, salvar, status, excluir); formulário que volta com o digitado; números como o Java imprime (`89.90`, `12.000000`) e como o Thymeleaf formata (`R$ 1.234,56`) | idem |
| `web/templates/produtos` | Lista e formulário portados, com o JS das unidades extras do antigo | **golden contra o antigo: 100 passos, 0 divergências** |
| `e2e/lib/golden.mjs`, `e2e/comparar-produtos.mjs` | Executor comum dos roteiros golden (aceita "diferença decidida" como a do D-03) e o roteiro de Produtos | idem |

**O que o golden descobriu nesta fatia** (e eu corrigi, todos fora do que um teste feito só pela leitura do código acharia): a mensagem de uma só leitura do antigo é uma **fila amarrada ao destino** com a regra de "consumir remove os iguais" (T-27); as listas do antigo ordenam **sem acento** em todas as colunas (T-29); "Selecione…" fica marcado quando a unidade volta em branco; um produto novo que falha depois de gravado volta como "Editar Produto" com o código queimado (T-28). Esses mesmos itens valem para todas as telas seguintes.

Itens da matriz: `feito` CAD-01 a CAD-07 (falta o E2E Playwright). **Diferença decidida (D-03):** excluir produto sem documento agora funciona (o antigo falhava sempre). **Não testado:** E2E Playwright, pixel a pixel, celular real; o preço sugerido por unidade (CAD-06) é do documento.

### Fatia 6: Usuários e Permissões
| Pacote / pasta | O que tem | Testes |
|---|---|---|
| `internal/cadastro/usuario.go` | Regras do `UsuarioService`: novo usuário com senha em BCrypt (fator 10), 21 linhas de direito (Consulta nos 5 módulos padrão) e perfil opcional na mesma transação; edição que só copia nome, login, status, vendedor e senha (em branco mantém); login único sem diferenciar maiúscula nem acento; `admin` protegido; vendedor vinculado precisa ser vendedor ativo; status; exclusão; matriz de permissões (módulo sem linha entra zerado), aplicar perfil (substitui tudo, não vale para o admin); código queimado quando o cadastro falha depois de gravar | 12 testes de integração em `internal/web/usuarios_test.go` + 1 unitário (ordem `CASE_INSENSITIVE_ORDER` do Java) |
| `internal/web` (`usuarios.go`) | Rotas `/usuarios/**` e `/usuarios/permissoes/**` (9 rotas) | idem |
| `web/templates/usuarios`, `web/templates/permissoes` | Lista, formulário e a matriz módulo × ação | **golden contra o antigo: 94 passos, 0 divergências** |
| `e2e/comparar-usuarios.mjs`, `e2e/lib/html.mjs` | Roteiro de Usuários e Permissões; o normalizador trata como iguais as duas redações da diferença decidida D-20 ("valem a partir do próximo login" × "valem na hora") | idem |

O golden desta fatia achou três coisas: (1) uma caixa de marcação enviada com valor **vazio** é erro de ligação no Spring — volta ao formulário (nas Unidades, que não tratam erro, dá 400) e o formulário mostra a caixa desmarcada; (2) a ordem dos vendedores do antigo vem do Java (`CASE_INSENSITIVE_ORDER`: ignora a caixa mas **não** o acento); (3) o código queimado também vale para `Usuarios`.
Reconferi os roteiros de Unidades/Pessoas (83 passos) e de Produtos (100) depois da mudança: **0 divergências**. Total comparado com o antigo até aqui: **277 passos**.

**Diferença decidida (D-20):** as permissões valem **na hora** (T-21), então os textos "valem a partir do próximo login do usuário" viraram "valem na hora…". Itens da matriz: `feito` CAD-12, CAD-13, CAD-14 (falta o E2E); `testado` SEG-13.
**Não testado:** E2E Playwright, pixel a pixel, celular real; o uso do vendedor vinculado no app e nos documentos vem com a API e os documentos.

### Fatia 7: Estoque, Documentos e o núcleo do financeiro
| Pacote / pasta | O que tem | Testes |
|---|---|---|
| `internal/estoque` | Entrada com custo médio ponderado, saída (saldo insuficiente barra), entrada sem mexer no custo; serviço não tem estoque; tudo na unidade base | via documentos |
| `internal/financeiro` (`nucleo.go`) | O que o documento usa do financeiro: gerar títulos de venda (a receber) e compra (a pagar) com parcelas (total exato, resto na última), vencimentos, categoria (escolhida → maior soma dos itens → do sistema), **baixa automática** com movimento no extrato, **taxa de cartão** como título PAGAR já baixado, desfazer tudo ao editar/excluir (baixa manual e conciliada bloqueiam), resumo para a tela. A baixa manual e o estorno vão reaproveitar `baixarInterno` | idem |
| `internal/documentos` | `Salvar`, `Atualizar`, `Excluir` na **ordem de verificações do antigo**, numa transação: classificação, serviços e comissão, segmento do item de maior valor, saldo de cada item, estoque, títulos; códigos queimados quando falha depois de gravar | 13 testes de integração (`documentos_test.go`) |
| `internal/web` (`documentos.go`, `estoque.go`) | Lista com filtros e escopo de vendedor, formulário (com o JS do antigo), visualizar, editar, excluir e a tela de estoque (números com o arredondamento HALF_EVEN do Java) | idem |
| `internal/dominio/formato.go` | `FormatarBR` = `#numbers.formatDecimal` do Thymeleaf (HALF_EVEN, qualquer tamanho) e `Escala` = o que o BigDecimal imprime | 2 testes unitários |
| `e2e/comparar-documentos.mjs` | **164 passos, 0 divergências** (e só a diferença decidida D-21): telas e mensagens dos 4 tipos, entradas e saídas, compras e vendas em todas as formas, edição, exclusão | + estado final |
| `e2e/despejo.mjs`, `tools/despejo`, `e2e/despejo-tabelas.json` | **Compara o banco inteiro** (13 tabelas) do antigo e do novo depois de cada roteiro: 27 documentos, 31 itens, 29 títulos, 10 baixas, 10 movimentos e o estoque batem linha a linha | todos os roteiros |

**O que o golden achou nesta fatia:** (1) o **defeito latente do antigo** (D-21): venda com fração de centavo em PIX/Dinheiro falha; (2) a ordem da lista de documentos na mesma data é pelo **código decrescente**; (3) o Jackson do antigo escreve as chaves do JSON das unidades em **ordem aleatória** (o comparador agora compara pelo conteúdo); (4) o cadastro do admin do antigo só tinha 10 linhas de direito até alguém abrir a tela de permissões (o novo cria as 21 já no boot; o comparador ignora o admin nessa tabela).
**Total comparado com o antigo até aqui: 441 passos** (83 + 100 + 94 + 164) **e as 13 tabelas do banco** depois de cada roteiro, **0 divergências** fora das diferenças decididas (D-03, D-20, D-21).

Itens da matriz: `testado` DOC-04, DOC-07, DOC-08, DOC-11, DOC-12, DOC-13, DOC-15, FIN-02, FIN-04, FIN-10; `feito` DOC-01, DOC-02, DOC-03, DOC-05, DOC-06, DOC-09, DOC-10, DOC-14, DOC-17, FIN-01, FIN-03, FIN-07.
**Não testado ainda:** E2E Playwright, o golden da **taxa de cartão** (precisa do cadastro de formas do financeiro), a venda a partir da agenda (D-22), a API (C), relatórios (DOC-18 a DOC-20), celular real.

## Próximas fatias (ordem proposta)
1. ~~Configuração por variáveis de ambiente e admin inicial~~ (feito na fatia 2).
2. ~~Servidor HTTP: sessão, CSRF, limite de login, permissões, layout e estáticos~~ (feito na fatia 3; falta o conteúdo do dashboard).
3. ~~Cadastros: unidades, pessoas, produtos, usuários e permissões~~ (fatias 4 a 6).
4. ~~Documentos + estoque + núcleo do financeiro~~ (fatia 7). Falta o **Financeiro** (títulos, baixa manual, estorno, contas, extrato, conciliação, transferências, cadastros, fluxo, DRE) e os **relatórios**.
5. **API `/api` e `/vendedor/api`** contra `docs/referencia/api/contrato.json`.
6. **Salão** (agenda, comissões), **metas, dashboard, BI, relatórios**.
7. **Backup/restore para o Google Drive**, Docker/Caddy, `cmd/migrar`, medição de RAM.

## Pendências de você
- **D-08**: como usar o rea.tools na seção ENGENHARIA e o que faltou no fim do pedido ("passo a passo de…").
- Mais adiante (**D-10**): um backup `.bak` do banco da VPS para ensaiar a migração.
