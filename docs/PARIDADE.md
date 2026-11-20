# Matriz de paridade — Áurea Concept (Java/SQL Server → Go/SQLite)

> **Regra:** nenhuma funcionalidade é dada como migrada sem **teste passando** e a coluna *Status* marcada. Estados: `pendente` → `feito` (implementado) → `testado` (teste automatizado verde).
> Todos os itens estão `pendente` porque a **Fase 1 (análise) termina aqui**; a Fase 2 só começa depois da aprovação desta matriz.
> **Tipos de teste:** `U` unitário (regra pura) · `I` integração Go (`httptest` + SQLite real) · `E` E2E Playwright (jornada, desktop/celular, acessibilidade, comparação visual com `docs/referencia/`) ·
> `C` contrato da API (comparação com `docs/referencia/api/contrato.json`) · `G` *golden*: mesmo roteiro (`e2e/semear.mjs`) rodado no antigo e no novo, saídas comparadas · `M` migração de dados · `S` script/operacional.
> **Referências:** `RN-…` e seções = [INVENTARIO.md](INVENTARIO.md); capturas = `docs/referencia/`.

Resumo: **144 itens** em 12 áreas (mais os itens descartados da seção M). Itens marcados **⚠** têm uma decisão pendente em [DUVIDAS.md](DUVIDAS.md).

## A. Plataforma e infraestrutura (INF)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| INF-01 | Binário único Go (net/http/chi), `html/template`, sem CGO, arquivos estáticos e templates embutidos | meta ≤ 512 MB | I, S | pendente |
| INF-02 | SQLite via `modernc.org/sqlite`: WAL, `busy_timeout`, `foreign_keys=ON`, **uma conexão de escrita**, transações em documento/estoque/financeiro | RN-DOC-06 | I | feito (I ✔) |
| INF-03 | Migrations versionadas embutidas (convertendo `schema.sql` + `02`–`13`), controle de versão, rodam no boot; script destrutivo nunca roda sozinho | §4 | I | feito (I ✔) |
| INF-04 | Configuração 100 % por variáveis de ambiente + `.env.example` comentado (PORT, DB_PATH, DOMINIO, COOKIE_SECURE, perfil prod, limites de login, backup GDRIVE_*, retenção, horário, admin inicial) | §3 | I | feito (U ✔ `internal/config` + `.env.example`; falta ligar ao `main`) |
| INF-05 | Perfil `prod`: cookie seguro, logs enxutos, recusa configuração insegura (sem senha padrão), shutdown gracioso, compressão | `application-prod.properties` | I | feito (cookie `Secure` e HSTS em prod I ✔; recusa de config insegura U ✔; log JSON e desligamento gracioso implementados sem teste automatizado; compressão fica no Caddy) |
| INF-06 | Healthcheck HTTP (`/login` 200 ou `/healthz`) e `HEALTHCHECK` no Docker | Dockerfile | S | feito (`/healthz` I ✔ e `aurea healthcheck` prontos; falta o `HEALTHCHECK` no Dockerfile) |
| INF-07 | Fuso `America/Sao_Paulo` (tzdata embutido) para "hoje", "mês atual", DRE e metas | §8-5 | U, I | feito (U ✔ fuso embutido; falta aplicar nas telas) |
| INF-08 | Estáticos com URL versionada por hash do conteúdo e cache de 365 dias em `prod` | §8-8 | I | testado (I ✔: toda URL de CSS/JS/imagem/vendor das telas vem com hash, responde 200, `Cache-Control` de 365 dias e 304 com ETag; hash errado dá 404) |
| INF-09 | Página de erro 404/500 simples em português mantendo o status HTTP ⚠ D-11 | screenshot `rota-inexistente` | E | pendente |
| INF-10 | Dockerfile multi-stage, imagem final mínima, usuário não-root | `Dockerfile` | S | pendente |
| INF-11 | `docker-compose.yml`: app + Caddy (HTTPS automático) + serviço de backup, volume do SQLite, `mem_limit` | `docker-compose.yml` | S | pendente |
| INF-12 | Scripts de deploy `backup.sh`, `restore.sh`, `atualizar.sh` adaptados (backup consistente do SQLite) | `deploy/` | S | pendente |
| INF-13 | Medição de RAM em repouso e sob uso, comparação com o Java (`docs/DESEMPENHO.md`) | meta | S | pendente |
| INF-14 | Teste de carga leve | meta | S | pendente |

## B. Segurança e acesso (SEG)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| SEG-01 | Login por formulário (`login`, `senha`), logout, redirecionamentos e mensagens de `/login` (erro, logout, expirada, bloqueado) | §1, §3 | I, E | pendente |
| SEG-02 | **Hashes BCrypt existentes continuam válidos** (`$2a$10$…`); novos hashes BCrypt fator 10 | RN-SEG-01 | U, I, M | feito (U ✔ com hashes reais do Java; faltam I, M) |
| SEG-03 | Sem senha padrão em produção: admin inicial por variável/primeiro boot ⚠ D-01 | pedido | I | feito (I ✔ com SQLite real: cria, recusa sem senha, recusa senha fraca, recusa hash do `Admin@123`; falta teste do boot completo) |
| SEG-04 | Usuário inativo não entra | RN-USR | I | testado (I ✔: inativo não entra; inativar derruba a sessão em curso) |
| SEG-05 | **Sessão única por usuário** (novo login expira a antiga → `/login?expired=true`), sessões em memória, uma instância ⚠ D-12 | §3 | I | testado (I ✔: novo login → a antiga vai a `/login?expired=true`; sessões persistidas no SQLite, sobrevivem a reinício ⚠ D-12) |
| SEG-06 | Sessão web 8 h; **app instalado** (`modoApp=1`) 30 dias sem uso com cookie persistente (`SESSAO_APP_DIAS`, 0 desliga) | `SessaoApp` | I | testado (I ✔: 8 h de inatividade no navegador; app instalado 30 dias com `Max-Age`; variável `SESSION_APP_DIAS`) |
| SEG-07 | Cookie `HttpOnly`, `SameSite=Lax`, `Secure` por `COOKIE_SECURE` | §3 | I | testado (I ✔: `HttpOnly`, `SameSite=Lax`, `Secure` por `COOKIE_SECURE` ou prod; id novo a cada login; só o SHA-256 vai ao banco) |
| SEG-08 | **CSRF** em todas as telas POST; desligado em `/api/**`; `/vendedor/api` exige CSRF (403 sem) | §2, §3 | I | feito (I ✔ nas telas, no login e no logout por POST; cabeçalho `X-CSRF-TOKEN` aceito; falta `/api` sem CSRF e `/vendedor/api`) |
| SEG-09 | **Limite de login**: 5 erros por IP+login / 30 por IP → bloqueio de 15 min (formulário e API, 429), janela e zeragem no sucesso, em memória | `LoginThrottle` | U, I, C | feito (U ✔ + I ✔: 5 por IP+login, 30 por IP, 15 min, zera no sucesso, IP do `X-Forwarded-For` atrás do Caddy; falta C e o 429 da API) |
| SEG-10 | 21 módulos × 4 direitos: tabela `Direito_Usuario`, authorities `MODULO_ACAO`, **`admin` sempre tem tudo** | `Permissoes` | U, I | testado (U ✔ + I ✔: o menu e o porteiro seguem os direitos lidos do banco a cada pedido) |
| SEG-11 | Mapa rota → módulo → ação (inclui `salvar` com/sem id; `/agenda/bloqueios|horarios`; `/vendedor`, `/vendedor/api`; BI e `/financeiro` validados no controller) | `PermissaoInterceptor` | U, I | testado (U ✔ 85 rotas + I ✔ no porteiro real: Produtos, Pessoas, BI, lucratividade) |
| SEG-12 | Negação: flash `erro` "Você não tem permissão para …" e redirect a `/dashboard` | §1 | I, E | feito (I ✔: mensagem por ação, de uma só leitura, volta ao `/dashboard`; falta E) |
| SEG-13 | Perfis prontos: Vendedor da loja, Profissional do salão, Recepção, Gerência (aplicar na criação e na tela de permissões, substitui tudo, não vale para admin) | `PerfisAcesso` | U, I | feito (U ✔; falta I) |
| SEG-14 | Escopo de vendas (`VENDAS_TODOS`): lista, abrir/editar/excluir, dashboard, relatório de documentos, BI/lucratividade bloqueados; sem vendedor vinculado = nada | `EscopoVendas` | I, E | pendente |
| SEG-15 | Escopo da agenda (`AGENDA_TODOS`): só a própria agenda; servidor ignora o profissional enviado | `AgendaEscopo` | I | pendente |
| SEG-16 | Tokens do app: 256 bits, só SHA-256 no banco, validade 30 dias, revogação, `ultimo_uso` a cada 10 min | `ApiTokenService` | U, I, C | pendente |
| SEG-17 | Cabeçalhos de segurança equivalentes aos padrões do Spring (nosniff, frame-options, cache-control) | framework | I | pendente |

## C. Cadastros (CAD)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| CAD-01 | Produtos: lista (busca só ativos, sem acento), novo, editar, validações (nome, unidade, preço ≥ 0,01, comissão, duração) | RN-PROD-01/06 | I, E, G | pendente |
| CAD-02 | Produto: ativar/inativar; ativar com preço 0 → aviso e volta à edição | RN-PROD-05 | I | pendente |
| CAD-03 | Produto: excluir ⚠ D-03 (hoje falha sempre com a linha de Estoque) | RN-PROD | I | pendente |
| CAD-04 | Produto serviço: sem Estoque, só vira serviço com saldo 0, zera comissão/duração se não for serviço | RN-PROD-03 | U, I | pendente |
| CAD-05 | Produto: unidade base válida e ativa; trocar base com extras exige remover antes | RN-PROD-02/04 | I | pendente |
| CAD-06 | Produto: unidades extras (sigla, fator ≤ 1.000.000, preço opcional), preço sugerido por unidade | RN-UND-02 | U, I | pendente (regra U ✔) |
| CAD-07 | Produto: categoria financeira de venda/compra, segmento | RN-FIN-03 | I | pendente |
| CAD-08 | Unidades de medida: CRUD, sigla única sem diferenciar maiúscula/acento, em maiúsculas, bloqueios de trocar/excluir em uso | RN-UND-01 | U, I, E | pendente |
| CAD-09 | Pessoas: lista com busca (nome/CPF/telefone), filtro de tipo e segmento | RN-PESSOA | I, E | pendente |
| CAD-10 | Pessoas: novo/editar/excluir; papéis (cliente, fornecedor, vendedor, funcionário); segmentos; nascimento | §1 | I, E | pendente |
| CAD-11 | CPF/CNPJ único comparando só dígitos (cadastro antigo repetido pode ser editado); telefone repetido só avisa | RN-PESSOA | U, I, C | pendente |
| CAD-12 | Usuários: lista, novo (senha obrigatória, perfil, vendedor vinculado ativo), editar (senha em branco mantém), status, excluir | RN-USR | I, E | pendente |
| CAD-13 | Login do usuário único sem diferenciar maiúscula; `admin` não muda de login nem inativa nem exclui | RN-USR | I | pendente |
| CAD-14 | Tela de permissões: matriz módulo × ação (checkboxes `inclusao_`…), aplicar perfil, módulos novos zerados | RN-PERM | I, E | pendente |
| CAD-15 | Segmentos (Loja de roupa, Salão) semeados e usados em produto/pessoa/documento/título/DRE | §4 | I | pendente |

## D. Documentos e estoque (DOC)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| DOC-01 | Lista de documentos com filtros tipo/período e escopo de vendedor; aviso "só minhas vendas" | §1, RN-ESCOPO | I, E, G | pendente |
| DOC-02 | Novo documento: COMPRA, VENDA, ENTRADA, SAIDA; pessoa, data, itens (produto, qtd, valor, unidade, profissional), forma, parcelas, 1º vencimento, categoria, vendedor, segmento | RN-DOC-01 | I, E, G | pendente |
| DOC-03 | Classificação obrigatória em ENTRADA/SAIDA (AJUSTE/TRANSFERENCIA/QUEBRA); QUEBRA só em SAIDA | RN-DOC-02 | U, I | pendente |
| DOC-04 | Item: qtd ≥ 0,001, valor ≥ 0,01, total = qtd × valor, `docto_valor` = soma ⚠ D-05 | RN-DOC-03 | U, I, G | pendente (regra U ✔) |
| DOC-05 | Segmento do documento (escolhido ou o do item de maior valor; nenhum = Geral) | RN-DOC-04 | U, I | pendente |
| DOC-06 | Serviços: só em venda/compra; comissão exige profissional; guarda % do momento; item não serviço sem profissional | RN-DOC-05 | U, I | pendente |
| DOC-07 | Estoque na unidade base: `qtdBase` (3 casas), `valorBase` (4 casas HALF_UP), fator gravado no item | RN-EST-01 | U, I, G | pendente (regra U ✔) |
| DOC-08 | **Custo médio ponderado** na entrada (4 casas HALF_UP); saída não altera custo; custo 0 se saldo ≤ 0 | RN-EST-02/03 | U, I, G | pendente (regra U ✔) |
| DOC-09 | Saída bloqueia estoque insuficiente (mensagem "Disponível | Solicitado"); estoque nunca negativo | RN-EST-03 | U, I, C | pendente |
| DOC-10 | Serviço não movimenta estoque | RN-EST-04 | U, I | pendente |
| DOC-11 | Salvar tudo na mesma transação (estoque + títulos); falha desfaz tudo | RN-DOC-06 | I | pendente |
| DOC-12 | Editar documento: refaz financeiro e estoque (reversão, validação de saldo, bloqueio por baixa manual/conciliada) | RN-DOC-07 | I, G | pendente |
| DOC-13 | Excluir documento: reverte estoque e financeiro; "estoque já consumido" bloqueia | RN-DOC-07 | I | pendente |
| DOC-14 | Visualizar documento (itens, totais, financeiro) | §5 | E, G | pendente |
| DOC-15 | Vendedor do documento: escolhe quem vê tudo; fixo no usuário para os demais; erro sem vínculo | RN-DOC-08 | I | pendente |
| DOC-16 | Lançar venda a partir da agenda (`/documentos/novo?agendamentoId=`, liga e conclui) | RN-DOC-09 | I, E | pendente |
| DOC-17 | Tela de estoque (saldo, custo, filtro baixo ≤ 5, valor total) | RN-EST-05 | I, E, G | pendente |
| DOC-18 | Relatório de estoque (todos/baixo/zerado) | §1 | I, G | pendente |
| DOC-19 | Relatório de documentos (tipo/período, total, escopo) | §1 | I, G | pendente |
| DOC-20 | Relatório de lucratividade (qtd, venda, custo médio, lucro; exige VENDAS_TODOS) | §1 | I, G | pendente |

## E. Financeiro (FIN)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| FIN-01 | Venda → títulos a receber; compra → a pagar; forma obrigatória/ativa/da natureza certa; baixa automática exige conta | RN-FIN-01 | U, I, G | pendente |
| FIN-02 | Parcelas: ROUND_DOWN + resto na última; vencimentos por dias; competência dia 1; descrição "Venda #N — Pessoa" | RN-FIN-02 | U, I, G | pendente (regra U ✔) |
| FIN-03 | Categoria do título (escolhida → maior soma do produto → do sistema); segmento do documento | RN-FIN-03 | U, I | pendente |
| FIN-04 | Baixa automática pela forma de pagamento (dinheiro/PIX) | RN-FIN-04 | I, G | pendente |
| FIN-05 | Baixa manual: parcial, juros, multa, desconto, conta ativa, validações, movimento no extrato, status | RN-BAIXA-01 | U, I, G | pendente (regra U ✔) |
| FIN-06 | Baixa em lote (saldo total, forma do título) | RN-BAIXA-01 | I | pendente |
| FIN-07 | **Taxa de cartão** vira título de despesa baixado; mensagem "Taxa de R$ x lançada como despesa." | RN-BAIXA-02 | U, I, G | pendente (regra U ✔) |
| FIN-08 | Estornar baixa (bloqueio por conciliação, desfaz taxa, recalcula status) | RN-ESTORNO | I, G | pendente |
| FIN-09 | Cancelar título (só sem baixa; taxa não cancela direto) | RN-ESTORNO | I | pendente |
| FIN-10 | Remoção de títulos ao editar/excluir documento (baixa automática some; manual/conciliada bloqueia) | RN-FIN-05 | I | pendente |
| FIN-11 | Título avulso: novo/editar, parcelas 1–60, `repetir`/`dividir`, mês a mês, segmento opcional, só sem baixa edita | RN-FIN-AV | U, I, G | pendente |
| FIN-12 | Lista de títulos: filtros natureza/status (abertos, vencidos, baixados, cancelados, todos)/período/busca; detalhe com baixas | §1 | I, E, G | pendente |
| FIN-13 | Contas: CRUD, status, excluir com bloqueios, saldo atual e conciliado | RN-CAD, RN-CONC | I, E | pendente |
| FIN-14 | Extrato (período padrão = mês), saldo anterior, saldo após cada linha, diferença com saldo informado | RN-CONC | U, I, G | pendente |
| FIN-15 | Conciliação (marcar/desmarcar lançamentos visíveis, contagem na mensagem) | RN-CONC | I | pendente |
| FIN-16 | Transferência entre contas + estorno (fora de DRE e fluxo; conta pode ficar negativa) | RN-TRF | U, I, G | pendente |
| FIN-17 | Cadastros: formas (taxa, parcelas, prazos, baixa automática, conta), tipos, categorias (do sistema protegidas) | RN-CAD | I, E | pendente |
| FIN-18 | Visão geral do financeiro (hoje, vencidos, 7 dias, próximos, saldos, receita/resultado) e redirecionamento por permissão | RN-FIN-VG | I, E, G | pendente |
| FIN-19 | **Fluxo de caixa** (realizado + previsto, diário até 62 dias/mensal, atrasados) | RN-FLUXO | U, I, G | pendente |
| FIN-20 | **DRE** competência e caixa, mês anterior ao lado, %, variação, filtro de segmento | RN-DRE | U, I, G | pendente |
| FIN-21 | Mensagens flash do financeiro (todas do catálogo) | `mensagens-antigo.txt` | I | pendente |

## F. Salão (SAL)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| SAL-01 | Serviços (produto sem estoque, % comissão, duração) na venda com profissional | RN-DOC-05 | I | pendente |
| SAL-02 | Tela de comissões (período, profissional, totais, escopo próprio) | RN-COM | U, I, G | pendente (regra U ✔) |
| SAL-03 | Agenda do dia: colunas por profissional, faixas lado a lado, indisponíveis, ocupação, janela do dia | RN-AGD | U, I, E | pendente |
| SAL-04 | Novo/editar agendamento (cliente ou avulso + telefone, serviços com duração e valor, obs, status) e avisos que não impedem | RN-AGD | U, I, E | pendente |
| SAL-05 | Mudar status (AGENDADO→CONFIRMADO→EM_ATENDIMENTO→CONCLUIDO, FALTOU, CANCELADO; FALTOU/CANCELADO liberam horário) | RN-AGD | I | pendente |
| SAL-06 | Excluir agendamento (proibido se virou venda) | RN-AGD | I | pendente |
| SAL-07 | Faturar: lançar venda do atendimento (liga e conclui) | RN-DOC-09 | I, E | pendente |
| SAL-08 | Bloqueios (salão todo ou profissional): criar/excluir; avisos | RN-AGD | I, E | pendente |
| SAL-09 | Expediente por profissional e dia da semana, com pausa; validações | RN-AGD | U, I, E | pendente |
| SAL-10 | Painel do salão no dashboard (hoje, próximo, ocupação dia/semana, faltas do mês; protegido por try/catch) | RN-DASH | U, I | pendente |
| SAL-11 | Link e mensagem de WhatsApp (confirmação, aniversário) | RN-DASH | U | pendente |

## G. Gestão: dashboard, metas, BI (GES)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| GES-01 | Dashboard: totais, vendas do mês (escopo), 10 últimos documentos, estoque baixo (6), por permissão | RN-DASH | I, E, G | pendente |
| GES-02 | Aniversariantes de hoje e 7 dias (29/02 → 28/02; WhatsApp) | RN-DASH | U, I | pendente |
| GES-03 | Metas: tela por mês, loja toda e por vendedor, salvar/apagar, progresso (ROUND_DOWN), barra no dashboard | RN-META | U, I, E, G | pendente (regra U ✔) |
| GES-04 | **BI — Painel Executivo** (KPIs, séries, top produtos, mix, período anterior, atalhos de período, ajuste quando sem dados) | RN-BI | U, I, E, G | pendente |
| GES-05 | **BI — Insights** (janela 90 dias, alertas de margem/cobertura/sem giro/zerado/baixo/capital parado, parâmetros) | RN-BI | U, I, E, G | pendente |
| GES-06 | Permissões do BI (executivo, insights, VENDAS_TODOS) e mensagens | RN-BI | I | pendente |
| GES-07 | Gráficos Chart.js com cores do tema (`aureaCores()`) | §5 | E | pendente |
| GES-08 | Dashboard: totais de estoque, atalhos e últimos lançamentos por permissão ⚠ D-17 (paridade primeiro, esconder depois) | RN-DASH | I, E | pendente |

## H. API do aplicativo — `/api/**` (API)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| API-01 | `POST /api/login` (400/401/403/429, token, `usuario`) | §2 | C, I | pendente |
| API-02 | `GET /api/me`, `POST /api/logout` (revoga) | §2 | C | pendente |
| API-03 | `GET /api/clientes` (busca sem acento, nome/CPF/telefone/código, limite ≤ 300) | §2 | C | pendente |
| API-04 | `POST /api/clientes` (201, validações, 409 CPF, aviso de telefone) | §2 | C | pendente |
| API-05 | `GET /api/segmentos` | §2 | C | pendente |
| API-06 | `GET /api/produtos` (com/sem/todos, busca, limite, sem serviços, sem custo, unidades extras) | §2 | C, G | pendente |
| API-07 | `GET /api/produtos/{id}/ultima-venda` (200/404) | §2 | C | pendente |
| API-08 | `GET /api/formas-pagamento` (descrição, parcelável, máx., à vista) | §2 | C | pendente |
| API-09 | `GET /api/pedidos`, `GET /api/pedidos/{id}` (só do vendedor; 404 para outro) | §2 | C | pendente |
| API-10 | `POST /api/pedidos` (todas as validações, venda pelo mesmo `DocumentoService`, 201) | RN-API-PED | C, I, G | pendente |
| API-11 | Erros `{"erro": "…"}`: 400 regra/JSON inválido, 401 token, 403, 409, 429, 500 | §2 | C | pendente |
| API-12 | Formato JSON idêntico (números com escala, datas ISO, ordem dos campos) ⚠ D-13 | `corpo_bruto` do contrato | C | pendente |
| API-13 | App Android (`mobile/`) continua funcionando sem alteração de código (só o endereço do servidor) | `mobile/` | C, M | pendente |

## I. Modo vendedor e app instalável (VEN / PWA)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| VEN-01 | `GET /vendedor` (SPA: pedidos, estoque, novo pedido, cliente rápido, item com unidade, última venda, detalhe) | §5 | E (celular) | pendente |
| VEN-02 | `/vendedor/api/**` = mesmas rotas e regras da `/api`, pela sessão + CSRF; `GET /vendedor/api/me`; 403 JSON sem permissão | §2 | C, I | pendente |
| VEN-03 | `vendedor-logica.js` (contas portadas do Kotlin) e os 23 testes Node continuam passando | `src/test/js` | U (Node) | pendente |
| VEN-04 | Rascunho do pedido em `localStorage`; entrada padrão do app instalado no Modo vendedor (`aurea-entrada`, só vendedor) | CLAUDE.md | E | pendente |
| PWA-01 | Manifesto, ícones, tags Apple, dica "Instalar no iPhone", `@media (display-mode: standalone)`; sem service worker | `static/img/pwa` | I, E | pendente |
| PWA-02 | Portar `verificar-pwa.sh` (73 checagens) e `verificar-vendedor.sh` (63) como testes automatizados | `deploy/` | I, E, S | pendente |

## J. Interface e design (UI)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| UI-01 | Layout base: menu lateral por permissão (grupos Loja/Salão/Gerência, financeiro), topo, rodapé, alertas flash | `layout/base.html` | I, E | feito (menu por permissão e mensagens I ✔; falta E e a comparação visual) |
| UI-02 | Tokens de tema `--silver*` (claro), tema escuro, 4 destaques, 3 densidades, preferências no `localStorage` (`aurea-*`) sem "piscar" (`tema-init.js`) | `style.css` | E | feito (CSS e JS copiados sem alteração; vistos no navegador; falta E) |
| UI-03 | Menu recolhível, grupos do menu, menu do usuário, paleta Ctrl+K (telas por permissão + aparência) | `main.js` | E | feito (`main.js` copiado sem alteração; iniciais e menu vistos no navegador; falta E) |
| UI-04 | **Comparação visual** de cada tela (49) contra `docs/referencia/`, desktop e celular, tolerância definida | `docs/referencia` | E | pendente |
| UI-05 | Acessibilidade básica (landmarks, rótulos, foco, contraste) em todas as telas | pedido | E | pendente |
| UI-06 | Fontes (Inter + Cormorant) e ícones/Chart.js: CDN ou servidos localmente ⚠ D-02, D-04 | §5 | E | feito (decisão D-04: Font Awesome 6.5.0, Chart.js 4.4.1, Inter e Cormorant servidos de `web/static/vendor`; no navegador nenhuma requisição externa; falta E) |
| UI-07 | Formulários: validação do navegador + mensagens do servidor, listas dinâmicas de itens, JS de documento/agenda | templates | E | pendente |
| UI-08 | Estilos de impressão | `style.css` | E | pendente |

## K. Dados e migração (MIG)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| MIG-01 | Ferramenta `cmd/migrar` SQL Server → SQLite (leitura do antigo, conversão de tipos, ids preservados) | §4 | M | pendente |
| MIG-02 | Conversão exata: DECIMAL(18,2/3/4/6), datas, horários, bit→bool, NULLs, collation | §4 | M, U | pendente |
| MIG-03 | Validação origem × destino: contagens por tabela, **soma de estoque, custo, saldos de contas, títulos em aberto**, tokens, permissões; relatório de divergências | pedido | M | pendente |
| MIG-04 | Hashes BCrypt e tokens (SHA-256) migrados como estão; sessões não migram | SEG-02 | M | pendente |
| MIG-05 | Tabelas `Script_Aplicado` e dados de demonstração do `schema.sql` **não** migram | §4 | M | pendente |
| MIG-06 | Ensaio completo com backup real da VPS (`.bak` fornecido pelo usuário) | etapa final | M | pendente |

## L. Backup, restore e operação (BKP)

| ID | Funcionalidade | Referência | Testes | Status |
|---|---|---|---|---|
| BKP-01 | Backup consistente (`VACUUM INTO` ou Online Backup API), gzip, timestamp | pedido | I, S | pendente |
| BKP-02 | Envio ao Google Drive via rclone (OAuth refresh token ou Drive compartilhado) | pedido | S | pendente |
| BKP-03 | Retenção configurável (ex.: 14 diários), agendamento (sidecar/cron) e log do resultado | pedido | S | pendente |
| BKP-04 | Alerta/log claro quando backup ou upload falha | pedido | S | pendente |
| BKP-05 | Restore documentado e **testado** (banco limpo + checagem de integridade + contagens) | pedido | I, S | pendente |

## M. Itens do projeto antigo descartados ou substituídos (registrados para não ficarem fora da matriz)

| ID | Item | Decisão |
|---|---|---|
| DES-01 | `carga/` (3 scripts SQL de carga) | descartar; os dados migram pela ferramenta |
| DES-02 | `dist/` (.bat do jar) | descartar |
| DES-03 | `deploy/init-db.sh`, `lib.sh`, `backup-db.sh`, `restore-db.sh` (SQL Server) | substituídos por INF-12 e BKP-* |
| DES-04 | `database/03_limpar_dados.sql` | descartado (nunca rodou sozinho; a limpeza **não** entra mais em nenhum comando) |
| DES-05 | Dados de exemplo do `schema.sql` | não migram nem entram nas migrations |
| DES-06 | Usuário `admin` com `Admin@123` do `schema.sql` | substituído por SEG-03 |
| DES-07 | Pacote Java `com.muralsemijoias`, Maven, Spring | encerrados com a migração |
