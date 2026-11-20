# Inventário do sistema antigo — Áurea Concept (Java/Spring Boot + SQL Server)

> **Fonte:** repositório `agnaldoojr/aureaconcept` no commit `7c3b487` (clone somente leitura em `C:\Users\agnal\aureaconcept-ref`).
> Tudo aqui foi extraído do código e do sistema rodando; nada do projeto antigo foi alterado.
> Referências `RN-nn` apontam para as regras de negócio da seção 6; `PAR-nn` para a matriz em [PARIDADE.md](PARIDADE.md).
> Itens que o código não deixa claro estão em [DUVIDAS.md](DUVIDAS.md).

Tamanho do código antigo: ~10,8 mil linhas de Java (190 arquivos em `com.muralsemijoias`), ~1 mil linhas de SQL (13 scripts), 49 templates
Thymeleaf (~3,5 mil linhas), `style.css` (1,9 mil linhas), JS de tela (`main.js`, `tema-init.js`, `vendedor*.js`), app Android em Kotlin (~1,3 mil linhas).

---

## 1. Rotas web (HTML, sessão + CSRF)

A permissão é decidida por **`PermissaoInterceptor`** a partir da URL (não por anotação). Regra: módulo = 1º segmento da URL; ação = pelo
nome da rota (`novo|nova|transferir` → Inclusão; `editar|status|baixar|baixar-lote|conciliar` → Atualização; `excluir|cancelar|estornar|estornar-transferencia`
→ Exclusão; `salvar` → Inclusão se o formulário não traz o código do registro, senão Atualização; qualquer outra → Consulta). Sem permissão: redireciona a
`/dashboard` com flash `erro` "Você não tem permissão para {acessar|incluir registros em|alterar registros em|excluir registros de} {rótulo do módulo}."
Dashboard, login, `/bi` e `/financeiro` (visão geral) não passam pelo interceptor (BI e visão geral conferem no controller).

| Método e rota | Módulo (ação) | Entrada | Saída / observações |
|---|---|---|---|
| `GET /login` | público | `error`, `logout`, `expired`, `bloqueado` | Tela de login; mensagens: "Login ou senha inválidos. Verifique seus dados.", "Você saiu do sistema com sucesso.", "Sua sessão expirou. Faça login novamente.", "Muitas tentativas de login. Aguarde alguns minutos e tente novamente." |
| `POST /login` | público | `login`, `senha`, `modoApp` (0/1) | Sucesso → `/dashboard` (zera contador IP+login; `modoApp=1` estende a sessão). Falha → `/login?error=true` (conta falha). IP bloqueado → `/login?bloqueado=true` |
| `GET /logout` | autenticado | — | Invalida sessão, apaga `JSESSIONID`, → `/login?logout=true` |
| `GET /` e `GET /dashboard` | autenticado (sem módulo) | — | Painel (ver RN-DASH) |
| `GET /produtos` | PRODUTOS (C) | `busca`, `status`… | Lista (a busca só devolve **ativos**; inativos só na lista completa) |
| `GET /produtos/novo` · `GET /produtos/editar/{id}` | PRODUTOS (I / A) | — | Formulário com unidades extras, categorias financeiras, segmento, serviço/comissão/duração |
| `POST /produtos/salvar` | PRODUTOS (I ou A) | campos `Produto` + linhas de unidades extras | Validações RN-PROD |
| `POST /produtos/status/{id}` | PRODUTOS (A) | `ativo` | Ativa/inativa (**ativar com preço 0 → aviso e volta à edição**, correção `9bbf242`) |
| `POST /produtos/excluir/{id}` | PRODUTOS (E) | — | "Não foi possível excluir…" (falha sempre se houver `Estoque`; ver DUVIDAS D-03) |
| `GET /unidades` · `/unidades/novo` · `/unidades/editar/{id}` | UNIDADES | — | Cadastro de unidades de medida |
| `POST /unidades/salvar` · `/status/{id}` · `/excluir/{id}` | UNIDADES | — | RN-UND |
| `GET /pessoas` | PESSOAS (C) | `busca`, `tipo`, `segmento` | Clientes/fornecedores/vendedores |
| `GET /pessoas/nova` · `/pessoas/editar/{id}` | PESSOAS (I/A) | — | Inclui segmentos e nascimento |
| `POST /pessoas/salvar` · `/pessoas/excluir/{id}` | PESSOAS | `segmentoIds` | RN-PESSOA (CPF duplicado barra; telefone só avisa) |
| `GET /documentos` | DOCUMENTOS (C) | `tipo`, `inicio`, `fim` | Escopo por vendedor (RN-ESCOPO) |
| `GET /documentos/novo` | DOCUMENTOS (I) | `agendamentoId` (opcional) | Pré-preenche cliente, serviços e profissional a partir da agenda |
| `POST /documentos/salvar` | DOCUMENTOS (I/A) | `Documento` + listas `produtoIds`, `quantidades`, `valores`, `unidades`, `profissionalIds`, `fpgCodigo`, `parcelas`, `primeiroVencimento`, `catCodigo`, `vendedorId`, `segmentoId`, `agendamentoId` | RN-DOC, RN-FIN |
| `GET /documentos/visualizar/{id}` · `/editar/{id}` | DOCUMENTOS (C / A) | — | Respeita escopo de vendas |
| `POST /documentos/excluir/{id}` | DOCUMENTOS (E) | — | Reverte estoque e financeiro |
| `GET /estoque` | ESTOQUE (C) | `filtro=baixo` | Saldo e custo médio por produto |
| `GET /relatorios` · `/relatorios/estoque` (`filtro=zerado|baixo`) · `/relatorios/documentos` (`tipo`,`inicio`,`fim`) | RELATORIOS (C) | — | Documentos respeitam escopo |
| `GET /relatorios/lucratividade` | RELATORIOS (C) **e** VENDAS_TODOS (C) | `inicio`, `fim`, `produtoId` | Soma todas as vendas; sem VENDAS_TODOS → dashboard com "Esta tela soma as vendas de todos os vendedores…" |
| `GET /comissoes` | COMISSOES (C) | `inicio`, `fim`, `profissionalId` | RN-COM |
| `GET /metas` · `POST /metas/salvar` | METAS (C / I) | `mes`, `vendedorIds[]`, `valores[]` | RN-META |
| `GET /agenda` (dia) | AGENDA (C) | `data`, `profissionalId` | Grade do dia (RN-AGD) |
| `GET /agenda/novo` · `/agenda/editar/{id}` · `GET /agenda/faturar/{id}` | AGENDA (I / A / C) | — | `faturar` redireciona a `/documentos/novo?agendamentoId=` |
| `POST /agenda/salvar` · `/status/{id}` · `/excluir/{id}` | AGENDA | `Dados` do agendamento | Avisos (info) nunca impedem |
| `GET /agenda/bloqueios` · `POST /agenda/bloqueios/salvar` · `/bloqueios/excluir/{id}` | AGENDA (C / A / E) | — | |
| `GET /agenda/horarios` · `POST /agenda/horarios/salvar` | AGENDA (C / A) | — | Expediente por profissional e dia da semana |
| `GET /bi` | BI_EXECUTIVO ou BI_INSIGHTS (C) **e** VENDAS_TODOS | `visao`, `periodo`, `inicio`, `fim`, `limite` (5), `diasSemGiro` (30) | RN-BI |
| `GET /financeiro` | qualquer FIN_* (C) | — | Visão geral (valida no controller) |
| `GET /financeiro/titulos` | FIN_TITULOS (C) | `natureza`, `status`, `inicio`, `fim`, `busca` | `status`: abertos (padrão), vencidos, baixados, cancelados, todos |
| `GET /financeiro/titulos/novo` · `/editar/{id}` · `/visualizar/{id}` | FIN_TITULOS | `natureza` | Avulso editável só sem baixa |
| `POST /financeiro/titulos/salvar` | FIN_TITULOS (I/A) | `titCodigo`, `natureza`, `pessoaId`, `catCodigo`, `fpgCodigo`, `descricao`, `emissao`, `competencia`, `vencimento`, `valor`, `parcelas`, `modo` (`repetir`/`dividir`), `segmentoId` | RN-FIN-AV |
| `POST /financeiro/titulos/baixar/{id}` | FIN_TITULOS (A) | `data`, `contaId`, `fpgId`, `valor`, `juros`, `multa`, `desconto`, `obs` | RN-BAIXA |
| `POST /financeiro/titulos/baixar-lote` | FIN_TITULOS (A) | `titIds[]`, `data`, `contaId` | Cada título pelo saldo total |
| `POST /financeiro/titulos/estornar/{bxId}` · `/cancelar/{id}` | FIN_TITULOS (E) | — | RN-ESTORNO |
| `GET /financeiro/contas` · `/novo` · `/editar/{id}` · `POST /salvar` · `/status/{id}` · `/excluir/{id}` | FIN_CONTAS | — | Saldo, conciliado |
| `GET /financeiro/contas/extrato/{id}` | FIN_CONTAS (C) | `inicio`, `fim`, `saldoExtrato` | Extrato com conciliação |
| `POST /financeiro/contas/conciliar/{id}` | FIN_CONTAS (A) | `visiveis[]`, `conciliados[]`, `inicio`, `fim`, `saldoExtrato` | |
| `GET /financeiro/contas/transferir` · `POST /transferir` · `POST /estornar-transferencia/{id}` | FIN_CONTAS (I / I / E) | `origem`, `destino`, `data`, `valor`, `observacao` | RN-TRF |
| `GET /financeiro/fluxo` | FIN_FLUXO (C) | `inicio`, `fim` | RN-FLUXO |
| `GET /financeiro/dre` | FIN_DRE (C) | `mes` (AAAA-MM), `regime` (`competencia`/`caixa`), `segmento` (vazio/`geral`/id) | RN-DRE |
| `GET/POST /financeiro/formas|tipos|categorias/…` (`novo`, `editar/{id}`, `salvar`, `status/{id}`, `excluir/{id}`) | FIN_CADASTROS | — | RN-CAD |
| `GET /usuarios` · `/novo` · `/editar/{id}` | USUARIOS | — | |
| `POST /usuarios/salvar` | USUARIOS (I/A) | `Usuario`, `novaSenha`, `vendedorId`, `perfil` | BCrypt; perfil pronto na criação |
| `POST /usuarios/status/{id}` · `/excluir/{id}` | USUARIOS | `ativo` | RN-USR |
| `GET /usuarios/permissoes/{id}` | PERMISSOES (C) | — | Matriz módulo × 4 ações |
| `POST /usuarios/permissoes/{id}/perfil` · `POST /usuarios/permissoes/{id}` | PERMISSOES (A) | `perfil` / mapa de checkboxes | Aplica perfil pronto ou salva marcação |
| `GET /vendedor` | APP_VENDAS (C) | — | Modo vendedor (SPA de celular) |
| `GET /css/**`, `/js/**`, `/img/**`, `/favicon.ico` | público | — | Estáticos com URL versionada por hash do conteúdo; 365 dias de cache em `prod` |

Todos os POST das telas exigem token CSRF (campo `_csrf` do Thymeleaf). Erros de bean validation voltam ao formulário com a mensagem do campo;
erros de regra voltam como flash `erro`; sucesso como flash `sucesso`; avisos como flash `info`. O catálogo completo (253 mensagens com arquivo e linha) está em
[referencia/mensagens-antigo.txt](referencia/mensagens-antigo.txt).

## 2. API do aplicativo — `/api/**` (sem sessão, sem CSRF, `Authorization: Bearer <token>`)

Erros: sempre `{"erro": "mensagem"}` (`Content-Type: application/json;charset=UTF-8`). Mapeamento: `ApiException(status)` → status próprio;
`IllegalArgumentException`/`IllegalStateException` → **400**; JSON ilegível ou tipo errado → **400** "Dados inválidos na requisição."; qualquer outra exceção →
**500** "Erro interno no servidor."; sem/ com token inválido → **401** "Sessão expirada ou inválida. Faça login novamente.".

| Rota | Permissão `APP_VENDAS` | Corpo / parâmetros | Resposta (JSON) |
|---|---|---|---|
| `POST /api/login` (público) | Consulta | `{login, senha, dispositivo}` | `{token, expiraEm, usuario:{codigo,nome,login,vendedor:{codigo,nome}\|null,podeLancar}}`. Erros: 400 "Informe o login e a senha."; **429** "Muitas tentativas de login. Aguarde alguns minutos e tente novamente."; 401 "Login ou senha inválidos."; 403 "Este usuário não tem acesso ao aplicativo de vendas. Peça ao administrador para liberar." |
| `GET /api/me` | Consulta | — | `UsuarioInfo` (`podeLancar` = Inclusão **e** vendedor vinculado) |
| `POST /api/logout` | — | — | 200 vazio; revoga o token |
| `GET /api/clientes?busca=&limite=100` | Consulta | máx. 300 | `[{codigo,nome,documento,telefone,nascimento,segmentos:[{codigo,nome}]}]` clientes **ativos**, busca sem acento em nome/CPF/telefone/código, ordem por nome sem acento |
| `POST /api/clientes` | **Inclusão** | `{nome,documento,telefone,nascimento,segmentos[]}` | **201** `{cliente, aviso}`; 400 nome obrigatório/≤150/doc>20/tel>20/nascimento futuro; **409** CPF duplicado; `aviso` "Atenção: já existe outra pessoa com este telefone." |
| `GET /api/segmentos` | Consulta | — | `[{codigo,nome}]` ativos |
| `GET /api/produtos?busca=&estoque=com\|sem\|todos&limite=100` | Consulta | máx. 300 | `[{codigo,nome,complemento,unidade,preco,estoque,unidades:[{sigla,fator,preco}]}]` ativos, **sem serviços**, **sem custo** |
| `GET /api/produtos/{id}/ultima-venda?clienteId=` | Consulta | — | `{data,quantidade,valor,unidade}` ou 404 "Sem vendas anteriores." |
| `GET /api/formas-pagamento` | Consulta | — | `[{codigo,nome,descricao,parcelavel,maxParcelas,aVista}]` formas de **recebimento** ativas |
| `GET /api/pedidos?busca=&limite=50` | Consulta | máx. 200 | `[PedidoDto]` do vendedor logado (sem itens); sem vendedor → `[]` |
| `GET /api/pedidos/{id}` | Consulta | — | `PedidoDto` com `itens[]`; pedido de outro vendedor → 404 "Pedido não encontrado." |
| `POST /api/pedidos` | **Inclusão** | `{clienteId,data,formaPagamentoId,parcelas,itens:[{produtoId,quantidade,valor,unidade}]}` | **201** `PedidoDto` `{codigo,data,clienteId,cliente,vendedor,valor,quantidadeItens,formaPagamento,parcelas,itens:[{produtoId,produto,unidade,quantidade,valor,total}]}` |

Regras do `POST /api/pedidos` (RN-API-PED): 409 sem vendedor vinculado; 400 sem itens/cliente/forma; cliente precisa ser **cliente ativo**; data não futura e não
anterior a 30 dias; item sem produto, produto inativo, quantidade ≤ 0, unidade inexistente no produto, valor ≤ 0; valor em branco → preço sugerido da unidade;
grava como `VENDA` com vendedor = vendedor do usuário e passa pelo **mesmo** `DocumentoService.salvar` (estoque, custo, títulos, baixa automática).

**`/vendedor/api/**`** (sessão do site + CSRF no POST): `VendedorApiController` **herda** `ApiVendasController`, portanto expõe as mesmas rotas
(`/vendedor/api/clientes|segmentos|produtos|formas-pagamento|pedidos…`) com as mesmas regras e JSON, mais `GET /vendedor/api/me`. Falta de permissão
responde 403 JSON (em vez de redirecionar). O app Android usa só `/api/**`.

## 3. Segurança

- **Senhas:** BCrypt, fator 10 (`$2a$10$…`). Hashes existentes precisam continuar válidos (RN-SEG-01).
- **Permissões:** 21 módulos × 4 direitos (Inclusão, Exclusão, Consulta, Atualização), tabela `Direito_Usuario` (uma linha por usuário × módulo). No login viram
  authorities `MODULO_ACAO`. **`admin` (login literal) sempre tem tudo.** Módulos (ordem da tela): PRODUTOS, UNIDADES, PESSOAS, DOCUMENTOS, VENDAS_TODOS*, COMISSOES*,
  AGENDA, AGENDA_TODOS*, ESTOQUE, RELATORIOS, BI_EXECUTIVO, BI_INSIGHTS, METAS, FIN_TITULOS, FIN_CONTAS, FIN_FLUXO, FIN_DRE, FIN_CADASTROS, APP_VENDAS, USUARIOS,
  PERMISSOES (`*` só Consulta tem efeito). Usuário novo nasce com Consulta em PRODUTOS, PESSOAS, DOCUMENTOS, ESTOQUE, RELATORIOS.
- **Perfis prontos** (`PerfisAcesso`): `VENDEDOR_LOJA` (consulta produtos/unidades/estoque; pessoas C+I+A; documentos C+I+A; APP_VENDAS C+I), `PROFISSIONAL_SALAO`
  (produtos/unidades/estoque C; pessoas C+I+A; documentos C+I+A; COMISSOES C; AGENDA C+I+A), `RECEPCAO` (AGENDA todas as ações; AGENDA_TODOS C; pessoas C+I+A; produtos C),
  `GERENCIA` (tudo menos PERMISSOES; VENDAS_TODOS/COMISSOES/AGENDA_TODOS só C; METAS C+I).
- **Escopo de vendas** (`EscopoVendas`): sem `VENDAS_TODOS` o usuário só vê documentos cujo **vendedor** é o vendedor vinculado ao seu usuário (lista, abrir/editar/excluir,
  dashboard, relatório de documentos); sem vendedor vinculado vê nada; BI e lucratividade ficam bloqueados. **Escopo da agenda** (`AgendaEscopo`): sem `AGENDA_TODOS`
  só a própria agenda (o servidor ignora o profissional enviado pela tela).
- **Sessão única por usuário** (`maximumSessions(1)`): um novo login **expira a sessão antiga** (a antiga cai em `/login?expired=true`). Sessões na memória (reiniciar derruba todos).
  Sessão web: 8 h (`SESSION_TIMEOUT`); **app instalado** (`modoApp=1`): 30 dias sem uso (`SESSAO_APP_DIAS`, 0 desliga) com cookie persistente `HttpOnly`, `SameSite=Lax`.
  Cookie: `HttpOnly`, `SameSite=Lax`, `Secure` conforme `COOKIE_SECURE` (padrão `true` em `prod`).
- **Limite de tentativas** (`LoginThrottle`, em memória): contador por **IP+login** (5 erros) e por **IP** (30 erros) → bloqueia o IP (ou IP+login) por **15 min**
  (`app.login.max-tentativas`, `max-tentativas-ip`, `bloqueio-minutos`). Login correto zera só IP+login. Janela: falha com mais de 15 min desde a última reinicia o contador.
  Vale para o formulário (`POST /login`, barrado antes de checar a senha) e para `POST /api/login` (responde 429).
- **Tokens do app:** 32 bytes aleatórios (`SecureRandom`), Base64 URL sem padding; no banco só o **SHA-256 em hex (64 chars)**; validade 30 dias; revogáveis;
  `ultimo_uso` atualizado no máximo a cada 10 min; usuário inativo/inexistente invalida a autenticação.
- **CSRF** ligado nas telas; desligado em `/api/**`. Cabeçalhos extras do Spring Security (nosniff, frame-options, cache-control) vêm do padrão do framework.
- **Usuário inativo não entra** (RN-USR-02). Login `admin` e senha inicial `Admin@123` vêm do `schema.sql` (**precisa ser removido do projeto novo**: ver DUVIDAS D-01).

## 4. Banco de dados (SQL Server, collation `Latin1_General_CI_AI`, 25 tabelas)

Criado por `database/schema.sql` + `02_` a `13_` (idempotentes; `03_limpar_dados.sql` é destrutivo e nunca roda sozinho). Tipos a respeitar na migração:
**quantidades** `DECIMAL(18,3)`; **preços e valores** `DECIMAL(18,2)`; **custo médio** `est_valor_custo DECIMAL(18,4)` (4 casas desde o script 06); **fator** `DECIMAL(18,6)`;
**taxas/percentuais** `DECIMAL(5,2)`; datas de negócio `DATE`; datas de auditoria `DATETIME`/`DATETIME2(0)`; horários `TIME(0)`; booleanos `BIT`.

| Tabela | Colunas principais (PK em **negrito**) | Constraints / índices |
|---|---|---|
| `Pessoas` | **ups_codigo**, ups_nome(150), ups_cpf(20), ups_telefone(20), ups_cliente, ups_fornecedor, ups_vendedor, ups_funcionario, ups_ativo, ups_criado_em, ups_nascimento | IX nome, IX cpf |
| `Pessoa_Segmento` | **(ps_ups_codigo, ps_seg_codigo)** | FK pessoa, FK segmento; IX segmento |
| `Segmento` | **seg_codigo**, seg_nome(60) UNIQUE, seg_ativo | semeia "Loja de roupa" e "Salão" |
| `Usuarios` | **usu_codigo**, usu_nome(150), usu_login(80) UNIQUE, usu_senha(255, BCrypt), usu_ativo, usu_criado_em, usu_ups_vendedor → Pessoas | login único **sem diferenciar maiúscula/acento** |
| `Direito_Usuario` | **dusu_codigo**, dusu_usu_codigo, dusu_descricao(100)=módulo, dusu_inclusao/exclusao/consulta/atualizacao | FK usuário |
| `Token_App` | **tok_codigo**, tok_usu_codigo (ON DELETE CASCADE), tok_hash CHAR(64) UNIQUE, tok_dispositivo(100), tok_criado_em, tok_expira_em, tok_ultimo_uso, tok_revogado | |
| `Produto` | **upr_codigo**, upr_nome(150), upr_complemento(255), upr_und(10), upr_preco, upr_ativo, upr_criado_em, upr_cat_venda/compra → Categoria_Financeira, upr_seg_codigo → Segmento, upr_servico, upr_comissao_perc, upr_duracao_min | CK comissão 0–100; CK duração 1–1440; IX nome |
| `Estoque` | **est_upr_codigo** (1:1 com produto), est_qtd(18,3), est_valor_custo(18,**4**) | FK produto; **serviço não tem linha** |
| `Unidade_Medida` | **und_codigo**, und_sigla(10) UNIQUE, und_descricao(60), und_ativo | semeia UN, CJ, PR, KG, MT + as usadas pelos produtos |
| `Produto_Unidade` | **pun_codigo**, pun_upr_codigo, pun_sigla(10), pun_fator(18,6) >0, pun_preco(18,2) nulo ou >0 | UNIQUE (produto, sigla) |
| `Documento` | **docto_codigo**, docto_tipo ∈ {COMPRA,VENDA,ENTRADA,SAIDA}, docto_data, docto_valor, docto_ups_codigo, docto_usu_codigo, docto_criado_em, docto_classificacao(20) ∈ {AJUSTE,TRANSFERENCIA,QUEBRA} ou nulo, docto_vendedor → Pessoas, docto_seg_codigo → Segmento | IX data, tipo, vendedor |
| `Itens_Documento` | **itemdocto_codigo**, itemdocto_docto_codigo, itemdocto_upr_codigo, itemdocto_upr_und, itemdocto_qtd(18,3), itemdocto_valor, itemdocto_total, itemdocto_fator(18,6) DEFAULT 1, itemdocto_prof_codigo → Pessoas, itemdocto_comissao_perc(5,2) | IX profissional |
| `Tipo_Titulo` | **tpt_codigo**, tpt_nome(80), tpt_natureza ∈ {RECEBER,PAGAR}, tpt_ativo | semeia 9 tipos |
| `Conta_Financeira` | **cfn_codigo**, cfn_nome(80), cfn_tipo ∈ {CAIXA,BANCO,MAQUININHA,OUTRA}, cfn_saldo_inicial, cfn_ativo, cfn_criado_em | semeia Caixa/Banco/Maquininha |
| `Categoria_Financeira` | **cat_codigo**, cat_nome(100), cat_tipo ∈ {RECEITA,DESPESA}, cat_grupo_dre ∈ {RECEITA_BRUTA, DEDUCOES, DESPESA_OPERACIONAL, OUTRAS_RECEITAS, OUTRAS_DESPESAS, NAO_DRE}, cat_codigo_sistema(30), cat_ativo | UNIQUE filtrado em cat_codigo_sistema (`VENDA_PRODUTOS`, `TAXA_CARTAO`, `COMPRA_MERCADORIA`: não excluíveis); semeia 12 categorias |
| `Forma_Pagamento` | **fpg_codigo**, fpg_nome(80), fpg_tpt_codigo, fpg_permite_parcelas, fpg_max_parcelas, fpg_taxa_percentual(5,2), fpg_dias_vencimento, fpg_intervalo_parcelas (30), fpg_baixa_automatica, fpg_cfn_codigo (conta padrão), fpg_nome_baixa(80), fpg_ativo | semeia 9 formas (Dinheiro, PIX, Débito 1,99%, Crédito até 12x 3,99%, Boleto, Crediário + 3 de pagamento) |
| `Titulo` | **tit_codigo**, tit_natureza, tit_origem ∈ {DOCUMENTO,AVULSO,TAXA}, tit_docto_codigo, tit_ups_codigo, tit_cat_codigo, tit_fpg_codigo, tit_tpt_codigo, tit_descricao(200), tit_parcela, tit_total_parcelas, tit_emissao, tit_competencia (sempre dia 1), tit_vencimento, tit_valor, tit_valor_baixado, tit_status ∈ {ABERTO,PARCIAL,BAIXADO,CANCELADO}, tit_baixa_origem (taxa → baixa que a gerou), tit_usu_codigo, tit_criado_em, tit_seg_codigo | IX vencimento, competência, documento, segmento |
| `Baixa` | **bx_codigo**, bx_tit_codigo, bx_data, bx_cfn_codigo, bx_fpg_codigo, bx_valor, bx_juros, bx_multa, bx_desconto, bx_taxa, bx_observacao(200), bx_automatica, bx_estornada, bx_estornada_em, bx_usu_codigo, bx_criado_em | IX data, título |
| `Movimento_Conta` | **mov_codigo**, mov_cfn_codigo, mov_data, mov_tipo ∈ {ENTRADA,SAIDA}, mov_valor, mov_descricao(200), mov_bx_codigo, mov_conciliado, mov_conciliado_em, mov_criado_em, mov_trf_codigo | IX (conta, data), IX transferência |
| `Transferencia_Conta` | **trf_codigo**, trf_data, trf_cfn_origem, trf_cfn_destino, trf_valor >0, trf_observacao(200), trf_usu_codigo, trf_estornada, trf_estornada_em, trf_criado_em | CK origem ≠ destino; IX data |
| `Meta_Venda` | **meta_codigo**, meta_mes (dia 1), meta_ups_vendedor (nulo = loja toda), meta_valor >0, meta_criado_em | UNIQUE (mes, vendedor); no SQL Server NULL conta como valor único (ver DUVIDAS D-07) |
| `Agendamento` | **agd_codigo**, agd_cliente → Pessoas, agd_cliente_nome(150), agd_cliente_telefone(20), agd_prof_codigo, agd_inicio, agd_fim, agd_status ∈ {AGENDADO,CONFIRMADO,EM_ATENDIMENTO,CONCLUIDO,FALTOU,CANCELADO}, agd_obs(500), agd_docto_codigo (ON DELETE SET NULL), agd_usu_codigo, agd_criado_em | CK fim>início; CK cliente ou nome avulso; IX (prof, início) |
| `Agendamento_Servico` | **ags_codigo**, ags_agd_codigo (ON DELETE CASCADE), ags_upr_codigo, ags_duracao_min >0, ags_valor ≥0 | |
| `Agenda_Horario` | **hor_codigo**, hor_prof_codigo, hor_dia_semana 1–7 (1 = segunda), hor_inicio, hor_fim, hor_pausa_inicio, hor_pausa_fim | UNIQUE (prof, dia); CK fim>início; CK pausa completa e dentro do expediente |
| `Agenda_Bloqueio` | **blq_codigo**, blq_prof_codigo (nulo = salão todo), blq_inicio, blq_fim, blq_motivo(150) | CK fim>início |
| `Script_Aplicado` | **nome**, aplicado_em | controle do `db-init` (infraestrutura, não é negócio) |

Dados de exemplo do `schema.sql` (8 produtos de semijoias, 8 pessoas, 1 compra): **não migram** (são de demonstração).

## 5. Telas (Thymeleaf → `html/template`)

49 templates em `src/main/resources/templates/`. Layout comum `layout/base.html` (menu lateral por permissão, topo, menu do usuário, paleta Ctrl+K, alertas flash).
`financeiro/_nav.html` é o submenu do financeiro. Telas:

- **Entrada:** `login`, `dashboard` (cards, últimos documentos, aniversariantes, estoque baixo, meta do mês, painel do salão).
- **Cadastros:** `produtos/{lista,form}`, `unidades/{lista,form}`, `pessoas/{lista,form}`, `usuarios/{lista,form}`, `permissoes/index`.
- **Operação:** `documentos/{lista,form,visualizar}`, `estoque/index`, `agenda/{dia,form,bloqueios,horarios}`, `comissoes/index`, `metas/index`, `vendedor/index` (SPA de celular).
- **Financeiro:** `financeiro/{visao,fluxo,dre}`, `titulos/{lista,form,detalhe}`, `contas/{lista,form,extrato,transferir}`, `formas|tipos|categorias/{lista,form}`.
- **Relatórios/BI:** `relatorios/{index,estoque,lucratividade,documentos}`, `bi/index` (Painel Executivo e Insights, com Chart.js).

Dependências externas das telas (CDN, carregadas no navegador): Font Awesome 6.5.0 (cdnjs), Chart.js 4.4.1 (cdnjs), Google Fonts (**Inter** no corpo + **Cormorant Garamond** em títulos/logo).
**Atenção:** a tela atual não usa a fonte "Jost" citada no pedido (ver DUVIDAS D-02). Estáticos próprios: `css/style.css` (tokens `--silver*`, tema claro/escuro, 4 destaques, 3 densidades),
`css/vendedor.css`, `js/main.js`, `js/tema-init.js`, `js/vendedor.js`, `js/vendedor-logica.js`, `img/logo-aurea.webp`, `img/pwa/*` (manifesto + ícones). Sem service worker (de propósito).

## 6. Regras de negócio (numeradas)

### Produtos, unidades e estoque
- **RN-PROD-01** Nome obrigatório (≤150), unidade obrigatória (≤10), **preço ≥ 0,01**, complemento ≤255; comissão 0–100; duração 1–1440 min.
- **RN-PROD-02** A unidade base precisa existir em `Unidade_Medida` e estar ativa (a unidade que o produto já tem é aceita mesmo inativa).
- **RN-PROD-03** Serviço (`upr_servico`): **sem linha em `Estoque`**; só vira serviço com saldo 0 ("Este produto ainda tem N em estoque. Zere o estoque…"); produto que não é serviço zera comissão e duração. Produto novo (não serviço) cria `Estoque` com 0/0.
- **RN-PROD-04** Trocar a unidade base com unidades extras exige removê-las antes ("Para trocar a unidade base (X), remova antes as unidades adicionais…").
- **RN-PROD-05** Ativar produto com preço < 0,01 → aviso "Defina o preço do produto … antes de ativar: clique em Editar." (volta à edição). Inativar sempre funciona.
- **RN-PROD-06** Busca de produtos na lista: nome contém (sem diferenciar maiúscula/acento) **e ativo**.
- **RN-UND-01** Unidade: sigla única sem diferenciar maiúscula, gravada em MAIÚSCULAS, ≤10, sem espaço; descrição ≤60; sigla em uso por produto não pode ser trocada; unidade em uso não pode ser excluída (só inativada).
- **RN-UND-02** Unidades extras do produto: fator > 0 e ≤ 1.000.000 (6 casas), preço opcional > 0, sigla não repete nem é a base; "preço sugerido" na unidade = preço da unidade ou preço do produto × fator (2 casas, HALF_UP).
- **RN-EST-01** O estoque fica sempre na **unidade base**. `qtdBase = qtd × fator` (3 casas HALF_UP); `valorBase = valor ÷ fator` (**4 casas HALF_UP**). O fator do momento é gravado no item.
- **RN-EST-02** **Entrada** (COMPRA/ENTRADA): `novoCusto = (saldo × custoAtual + qtdBase × valorBase) ÷ (saldo + qtdBase)` com 4 casas HALF_UP; novo saldo ≤ 0 → custo 0.
- **RN-EST-03** **Saída** (VENDA/SAIDA): bloqueia se `saldo < qtdBase` ("Estoque insuficiente para 'X'. Disponível: A | Solicitado: B"); saída **não altera o custo médio**; estoque nunca fica negativo.
- **RN-EST-04** Serviço não movimenta estoque (todas as operações de estoque ignoram serviço).
- **RN-EST-05** Estoque baixo = saldo ≤ 5 (dashboard e relatório; o BI tem limite configurável, padrão 5). Valor em estoque = Σ(saldo × custo).

### Documentos
- **RN-DOC-01** Tipos: COMPRA, VENDA (financeiros) e ENTRADA, SAIDA (estoque, sem título). Pelo menos 1 item; pessoa e data obrigatórias; usuário logado é gravado.
- **RN-DOC-02** ENTRADA/SAIDA exigem classificação **AJUSTE, TRANSFERENCIA ou QUEBRA**; QUEBRA só em SAIDA ("Quebra só pode ser lançada em uma saída."); compra/venda zeram a classificação.
- **RN-DOC-03** Item: quantidade ≥ 0,001; valor ≥ 0,01; `itemdocto_total = qtd × valor` (gravado em 2 casas); `docto_valor = Σ qtd×valor` dos itens (soma sem arredondar, arredondada uma vez ao gravar; ver DUVIDAS D-05).
- **RN-DOC-04** Segmento do documento: o escolhido na tela (precisa estar ativo) ou, em branco, o do item de **maior valor** (qtd×valor) entre os produtos com segmento; nenhum → sem segmento ("Geral").
- **RN-DOC-05** Serviço só entra em VENDA/COMPRA (não em ENTRADA/SAIDA). Na venda, serviço com comissão > 0 exige profissional; o item guarda o % do momento; item que não é serviço vendido fica sem profissional e sem comissão.
- **RN-DOC-06** Salvar: valida saldo de saída → grava → movimenta estoque (entrada com custo / saída) → gera títulos, tudo na **mesma transação** (falha em qualquer ponto desfaz tudo).
- **RN-DOC-07** Editar: refaz o financeiro (baixa manual ou conciliada bloqueia), valida que a entrada original ainda pode ser revertida (saldo ≥ qtd), reverte (`saída` simples para entradas, sem mexer no custo; `entradaSemAlterarCusto` para saídas), regrava e reaplica. Excluir: mesmo bloqueio e reversão ("Não é possível excluir: o estoque de 'X' já foi consumido…").
- **RN-DOC-08** Vendedor do documento: quem tem VENDAS_TODOS escolhe (precisa ser vendedor); quem não tem usa o vendedor vinculado ao próprio usuário (sem vínculo: "Seu usuário não tem vendedor vinculado…").
- **RN-ESCOPO** Sem VENDAS_TODOS: lista, relatório, dashboard, abrir/editar/excluir só dos próprios documentos; profissional dos serviços é sempre o vendedor do documento.
- **RN-DOC-09** "Lançar venda" da agenda: pré-preenche tipo VENDA, cliente, serviços (qtd 1, valor do agendamento ou preço do serviço), profissional; ao salvar liga a venda ao atendimento e o conclui; cliente avulso exige escolher/cadastrar pessoa.

### Financeiro
- **RN-FIN-01** Venda gera título **a receber**; compra, **a pagar**. Forma de pagamento obrigatória, ativa e da natureza certa; forma com baixa automática exige conta padrão.
- **RN-FIN-02** Parcelas: forma não parcelável ou baixa automática → 1 parcela; senão 1..`fpg_max_parcelas`. `valorParcela = total ÷ n` (2 casas, **ROUND_DOWN**); a **última parcela leva o resto**. Vencimento base: baixa automática → data do documento; senão `primeiroVencimento` informado ou data + `fpg_dias_vencimento`; parcela i = base + i × `fpg_intervalo_parcelas` dias. Competência = dia 1 do mês da data do documento; emissão = data do documento. Descrição "Venda #N — Pessoa" / "Compra #N — Pessoa".
- **RN-FIN-03** Categoria do título: a escolhida na tela; senão a categoria financeira do produto (de venda para receber, de compra para pagar) com **maior soma de total**; senão a do sistema (`VENDA_PRODUTOS` / `COMPRA_MERCADORIA`). Segmento do título = segmento do documento.
- **RN-FIN-04** Baixa automática: cria baixa (data do documento, conta padrão, `bx_automatica`) para cada parcela.
- **RN-BAIXA-01** Baixa manual: valor > 0 e ≤ saldo; juros/multa/desconto ≥ 0; desconto ≤ valor+juros+multa; título aberto/parcial; conta ativa. Valores a 2 casas HALF_UP. Movimento no extrato = valor + juros + multa − desconto (entrada se receber, saída se pagar). Status: baixado se `valor_baixado ≥ valor`, parcial se > 0, aberto se 0 (cancelado não muda). **Baixa em lote**: cada título pelo saldo total, forma do próprio título.
- **RN-BAIXA-02** **Taxa de cartão** (só título a receber com forma de taxa > 0): `taxa = valor × % ÷ 100` (2 casas HALF_UP) vira **título PAGAR origem TAXA** (categoria `TAXA_CARTAO`, descrição "Taxa {forma} — título #N") já baixado na mesma conta ("Taxa automática — {forma}"); `bx_taxa` guarda o valor.
- **RN-ESTORNO** Estornar baixa: bloqueia se o movimento (ou o da taxa filha) já foi **conciliado**; apaga movimentos, marca a baixa estornada, reduz `valor_baixado` (nunca abaixo de 0), recalcula o status e **estorna a baixa da taxa e cancela o título de taxa**. Cancelar título: só sem baixa; taxa só se desfaz estornando a baixa de origem.
- **RN-FIN-05** Remover títulos de documento (ao editar/excluir): baixas automáticas somem junto; baixa manual → "O documento tem baixa lançada manualmente no título #N. Estorne a baixa…"; conciliado → "O lançamento da baixa #N já foi conciliado…".
- **RN-FIN-AV** Título avulso: descrição, valor > 0, vencimento e categoria obrigatórios; pessoa opcional; segmento opcional (nulo = Geral); parcelas 1..60 (`modo=dividir` divide o valor com ROUND_DOWN e resto na última; `repetir` repete o valor); vencimento e competência avançam **mês a mês**; só avulso sem baixa e não cancelado pode ser editado.
- **RN-TRF** Transferência: contas diferentes e ativas, valor > 0 (2 casas), observação ≤200; grava saída na origem ("Transferência para X") e entrada no destino ("Transferência de X"); **não** entra em DRE nem em fluxo de caixa; conta pode ficar negativa; estorno apaga os dois movimentos e mantém o registro, bloqueado se alguma ponta foi conciliada.
- **RN-CONC** Conciliação: dos lançamentos visíveis, marcados ficam conciliados (com data/hora) e desmarcados deixam de estar. Saldo da conta = inicial + Σ movimentos (entrada +, saída −); saldo conciliado = inicial + Σ movimentos conciliados; diferença da tela = saldo informado do extrato − saldo conciliado. Extrato padrão: mês corrente; início > fim troca as datas.
- **RN-FLUXO** Fluxo de caixa: realizado (movimentos **sem transferência**) + previsto (saldo dos títulos em aberto/parcial pelo vencimento; vencido entra no dia de hoje; antes do início vai para o início). Diário até 62 dias, mensal acima; saldo inicial = Σ saldos iniciais das contas + movimentos antes do início (inclui transferências, que se anulam). Atrasado a receber/pagar separado.
- **RN-DRE** Receita bruta − deduções (taxas, descontos concedidos) = receita líquida − CMV = lucro bruto − despesas operacionais = resultado operacional ± outras (juros/multas, descontos obtidos, perdas) = resultado líquido, **com o mês anterior ao lado**, % da receita e variação. **Competência:** títulos não cancelados pela data de competência; CMV = Σ(custo médio atual × qtdBase) dos itens de VENDA; perdas = itens de SAIDA/QUEBRA. **Caixa:** baixas não estornadas pela data da baixa; no lugar do CMV mostra "Compras de mercadoria pagas". Compra de mercadoria não é despesa. Sinal natural: receita de "a receber" e despesa de "a pagar"; o contrário é estorno. Filtro de segmento: todos | `geral` | id.
- **RN-CAD** Cadastros: tipo (nome, natureza), conta (nome, tipo, saldo inicial; excluir só sem movimentos/transferências/formas), categoria (nome, tipo, linha da DRE; **categorias do sistema só renomeiam, não inativam nem excluem**), forma (taxa 0–100 só para receber; prazos ≥ 0; parcelas 1–60; baixa automática exige conta e ganha nome padrão). Excluir item em uso → "…Inative-o em vez de excluir.".
- **RN-FIN-VG** Visão geral: a receber/a pagar hoje, vencidos (valor e quantidade), próximos 7 dias, 8 próximos títulos, saldo por conta (se FIN_CONTAS), receita e resultado do mês (se FIN_DRE); sem FIN_TITULOS redireciona à primeira área permitida.

### Pessoas, usuários, comissão, metas, agenda, dashboard, BI
- **RN-PESSOA** Nome obrigatório (≤150); CPF/CNPJ ≤20 e **único comparando só dígitos** (quem já tinha CPF repetido pode ser editado sem trocá-lo); telefone repetido só **avisa**; segmento inativo só aceito se a pessoa já o tinha; nascimento não pode ser futuro (API).
- **RN-USR** Login único (sem diferenciar maiúscula); novo usuário exige senha (BCrypt 10) e nasce com 1 linha de direito por módulo (Consulta nos 5 padrão); edição só copia nome, login, status, vendedor e senha (senha em branco mantém); `admin` não muda de login, não inativa nem exclui; vendedor vinculado precisa ser vendedor **ativo**; usuário inativo não entra.
- **RN-PERM** Tela de permissões: checkboxes `inclusao_|exclusao_|consulta_|atualizacao_` + módulo; aplicar perfil **substitui todas** as permissões (não vale para admin); módulos novos aparecem zerados.
- **RN-COM** Comissão do item = `total do item × % ÷ 100` (2 casas HALF_UP); só itens de VENDA com `%` gravado e profissional; agrupa por profissional; escopo próprio sem VENDAS_TODOS. Pagamento da comissão **não** é controlado.
- **RN-META** Meta por mês (dia 1) e vendedor (nulo = loja toda); valor vazio ou 0 apaga; negativo é erro; só vendedor ativo; progresso = `realizado × 100 ÷ meta` (0 casas, **ROUND_DOWN**), barra limitada a 0–100, falta nunca negativa; realizado = Σ docto_valor de VENDA/SAIDA no mês.
- **RN-AGD** Profissional = pessoa **vendedor ativo do segmento Salão** (comparação sem acento). Expediente por profissional e dia da semana (1 = segunda), com pausa; dia sem linha = folga. Duração livre (padrão do serviço, 30 se vazia); `fim = início + Σ durações`. Estados: AGENDADO → CONFIRMADO → EM_ATENDIMENTO → CONCLUIDO, FALTOU, CANCELADO (FALTOU e CANCELADO **liberam o horário**; CANCELADO some da grade). Sobreposição, bloqueio e fora do expediente **só avisam** ("Sobrepõe o atendimento de…", "Horário bloqueado…", "Fora do expediente de…", "não tem expediente cadastrado neste dia da semana."). Cliente cadastrado **ou** nome avulso (+ telefone para WhatsApp). Só serviço pode ser agendado. Excluir atendimento que virou venda é proibido ("Use Cancelar"). Grade: colunas por profissional, faixas lado a lado para sobreposição, trechos indisponíveis (fora do expediente, pausa, bloqueio), janela do dia arredondada à hora (mínimo 4 h; padrão 8–19 h). Ocupação = minutos atendidos ÷ minutos úteis (expediente − pausa − bloqueios), em %. Bloqueio: início < fim, profissional opcional (nulo = salão todo). Horários: fim > início; pausa completa e dentro do expediente. Mensagem de confirmação de WhatsApp pronta.
- **RN-DASH** Total de produtos **ativos**, quantidade e valor em estoque, vendas do mês (escopo próprio sem VENDAS_TODOS), 10 últimos documentos, aniversariantes de hoje e dos próximos 7 dias (29/02 vira 28/02 em ano comum; WhatsApp: só dígitos, prefixo 55, aceita 10 ou 11 dígitos), 6 produtos ativos com estoque baixo, barra da meta do mês (loja toda para quem vê tudo, a própria para vendedor), painel do salão (atendimentos de hoje, próximo, ocupação do dia e da semana, faltas do mês — **protegido por try/catch** para nunca derrubar o dashboard).
- **RN-BI** Painel Executivo (faturamento, lucro bruto estimado, margem, ticket médio, compras, giro; série diária até 62 dias senão mensal; top produtos; mix por operação; comparação com o período anterior de mesma duração) e Insights (janela de 90 dias; alertas: margem de tabela < 20 %, cobertura < 15 dias, sem giro por N dias, estoque zerado/baixo, capital parado). Venda = VENDA ou SAIDA; compra = COMPRA ou ENTRADA; **custo = custo médio atual** do estoque. Exige VENDAS_TODOS.
  - **Painel Executivo** (`/bi`, atalhos `periodo` = `mes` | `30` | `90` (padrão) | `365`, ou `inicio`/`fim`; `fim` vazio = hoje; `inicio` vazio = fim − 89 dias; início > fim troca; no máximo 5 anos): KPIs = faturamento (Σ docto_valor de VENDA/SAIDA e nº de vendas), lucro bruto estimado (faturamento − Σ custo médio atual × qtdBase), margem bruta (%, 1 casa), ticket médio (2 casas), compras (VENDA/SAIDA ↔ COMPRA/ENTRADA), giro (custo das vendas ÷ valor do estoque, 2 casas, "x"); cada KPI traz a variação contra o período anterior de **mesma duração** (▲/▼, em % ou "p.p.", "sem base de comparação", "= estável"; compras é "neutro"). Série diária (≤ 62 dias) ou mensal (`MM/yyyy`) de vendas e compras; top 10 produtos por receita (qtd, receita, lucro, margem, participação), top 5 no gráfico; mix por tipo (Venda, Saída, Compra, Entrada). Na abertura sem dados no período e com último lançamento anterior ao início, ajusta para os 90 dias que terminam no último lançamento (`periodoAjustado`). Data/número inválido na URL volta a `/bi`.
  - **Insights** (`?visao=insights&limite=5&diasSemGiro=30`; `limite` 0–1000, `diasSemGiro` 1–365): janela de **90 dias**; por produto **ativo** (com linha de estoque) calcula cobertura = saldo ÷ (vendido em 90 dias ÷ 90, 6 casas), arredondada a 0 casas, só com saldo e vendas > 0; margem de tabela = (preço − custo) × 100 ÷ preço (1 casa) só com custo e preço > 0; sinalizadores e gravidade: *Sem estoque* (4), *Estoque baixo* (saldo ≤ limite; 3), *Cobertura curta* (< 15 dias, sem ser zerado/baixo), *Sem giro* / *Nunca vendido* (sem entrada recente e última venda antes do corte; 1; soma o capital parado = saldo × custo), *Preço abaixo do custo* (margem < 0; 4), *Margem baixa* (< 20 %), ou *OK*; ordena por gravidade (desc) e nome. Cartões, do mais grave ao informativo: sem estoque, preço abaixo do custo, estoque baixo, cobertura curta, margem baixa, sem giro (com capital parado), produto líder em receita (com participação, margem e aviso se a cobertura < 30 dias), melhor dia da semana (% do faturamento), projeção do mês (acumulado × dias do mês ÷ dia atual, 2 casas; e mês anterior), ausência de vendas, ou "Nenhum alerta crítico". Textos listam até 4 nomes e "e mais N". Moeda no formato pt-BR (`R$ 1.234,56` com espaço sem quebra); percentuais com vírgula.
- **RN-API-PED** Ver seção 2. **RN-SEG** Ver seção 3.

## 7. Pastas auxiliares

| Pasta | O que faz | Destino |
|---|---|---|
| `carga/` | 3 scripts SQL de **uso único** (carga inicial de 143 produtos como entrada de estoque, lote 2 com +16, ajuste de custo R$ 1,00 e saldo calça +9 / short +10). Ficam fora de `database/` de propósito | **Descartar** (a carga já foi aplicada na VPS; os dados migram pela ferramenta SQL Server → SQLite). Referência histórica apenas |
| `deploy/` | `Caddyfile`, `lib.sh`, `init-db.sh` (cria o banco e aplica scripts), `backup(-db).sh`, `restore(-db).sh`, `atualizar.sh`, `verificar-pwa.sh` (73 checagens), `verificar-vendedor.sh` (63 checagens) | **Migrar** `Caddyfile`, backup/restore/atualizar (adaptados ao SQLite); **portar** os dois `verificar-*.sh` como testes de integração/E2E; descartar `init-db.sh` e `lib.sh` (SQL Server) |
| `dist/` | 4 `.bat` + `aurea.ico` para abrir o `.jar` no Windows | **Descartar** (substituído pelo binário Go) |
| `mobile/` | App Android (Kotlin/Compose): login, estoque, pedidos, cliente rápido, item com unidade, última venda; usa `/api/**` com token | **Não alterar**: só muda o endereço do servidor na etapa final. É o oráculo do **contrato da API** |
| `database/` | `schema.sql` + `02`–`13` + `03_limpar` | **Converter** para migrations SQLite versionadas (embutidas no binário); `03_limpar` nunca roda sozinho |
| `Dockerfile`, `docker-compose.yml`, `.env.example` | Java + SQL Server Express + Caddy | **Refazer** em Go + SQLite + Caddy + sidecar de backup |

## 8. Comportamentos sutis que a migração precisa reproduzir

1. **Busca e comparação sem acento e sem maiúscula** (collation CI_AI no banco; `LOWER(...) LIKE`; `IgnoreCase`): o SQLite compara literalmente, então precisa de normalização própria.
2. **Unicidade de login/sigla/segmento sem diferenciar maiúscula e acento** (o índice único do SQL Server é CI_AI).
3. **Arredondamentos explícitos:** parcela `ROUND_DOWN` + resto na última; taxa e comissão `HALF_UP`; valorBase 4 casas; qtdBase 3 casas; progresso de meta `ROUND_DOWN`; `docto_valor` soma sem arredondar e arredonda ao gravar.
4. **Mês a mês** nos títulos avulsos (`plusMonths`, que ajusta 31 → último dia do mês) × **dias** (`plusDays`) nas parcelas de documento.
5. **Datas:** "hoje" e "mês atual" usam o fuso do servidor (`TZ=America/Sao_Paulo` no contêiner).
6. **Sessão única por usuário expira a antiga** (não bloqueia a nova); sessões e limite de login **na memória** (uma instância só).
7. **Flash messages** (`sucesso`, `erro`, `info`) sobrevivem a exatamente um redirect.
8. **Estáticos versionados por hash do conteúdo** (`style-<hash>.css`) e cache de 365 dias em produção.
9. **JSON** do app: `BigDecimal` serializado como número com a escala do banco (ex.: `89.90`, `10.000`); datas ISO (`yyyy-MM-dd`), `expiraEm` ISO com frações; ver captura real em [referencia/api/](referencia/api/).
10. **Interceptor de permissão** olha o *caminho*; rotas novas precisam entrar na tabela módulo × ação.
