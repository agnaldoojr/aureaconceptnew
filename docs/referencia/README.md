# Referências capturadas do sistema antigo

Geradas por `e2e/semear.mjs` + `e2e/capturar-referencia.mjs` + `e2e/capturar-api.mjs` contra o sistema **antigo** (Java, commit `7c3b487`) em um banco descartável, em 2026-11-20.
Desktop = 1366×900 (página inteira); celular = 390×844 com escala 2 (só admin e vendedor). Navegador: Microsoft Edge 157 via Playwright.

| Pasta | Capturas | Observação |
|---|---|---|
| `anonimo/desktop/` | 2 | tela de login (normal e com erro) |
| `anonimo/mobile/` | 2 | idem |
| `admin/desktop/` | 52 | todas as telas + tema escuro + destaque rosa/compacto |
| `admin/mobile/` | 49 | todas as telas em celular |
| `vendedor1/desktop/` | 6 | Vendedor da loja (escopo próprio, BI negado, Modo vendedor) |
| `vendedor1/mobile/` | 6 | Modo vendedor e telas do vendedor no celular |
| `gerente1/desktop/` | 5 | Gerência (sem permissões de usuários) |
| `salao1/desktop/` | 5 | Profissional do salão (própria agenda e comissão) |
| `recepcao1/desktop/` | 5 | Recepção (agenda de todos, sem vendas) |

**Total: 132 capturas**; 130 respondem 200, as 2 rotas inexistentes respondem 404 (página crua do Tomcat) e **nenhuma tela real gera erro de console**.

## Telas do perfil admin (ordem das capturas)

| # | Nome | Rota |
|---|---|---|
| 1 | dashboard | `/dashboard` |
| 2 | produtos-lista | `/produtos` |
| 3 | produtos-novo | `/produtos/novo` |
| 4 | produtos-editar | `/produtos/editar/1` |
| 5 | unidades-lista | `/unidades` |
| 6 | unidades-novo | `/unidades/novo` |
| 7 | pessoas-lista | `/pessoas` |
| 8 | pessoas-nova | `/pessoas/nova` |
| 9 | pessoas-editar | `/pessoas/editar/1` |
| 10 | usuarios-lista | `/usuarios` |
| 11 | usuarios-novo | `/usuarios/novo` |
| 12 | usuarios-permissoes | `/usuarios/permissoes/2` |
| 13 | documentos-lista | `/documentos` |
| 14 | documentos-novo | `/documentos/novo` |
| 15 | documentos-visualizar | `/documentos/visualizar/11` |
| 16 | documentos-editar-venda | `/documentos/editar/11` |
| 17 | estoque | `/estoque` |
| 18 | estoque-baixo | `/estoque?filtro=baixo` |
| 19 | relatorios-indice | `/relatorios` |
| 20 | relatorios-estoque | `/relatorios/estoque` |
| 21 | relatorios-lucratividade | `/relatorios/lucratividade` |
| 22 | relatorios-documentos | `/relatorios/documentos` |
| 23 | comissoes | `/comissoes` |
| 24 | metas | `/metas` |
| 25 | agenda-dia | `/agenda` |
| 26 | agenda-novo | `/agenda/novo` |
| 27 | agenda-editar | `/agenda/editar/1` |
| 28 | agenda-bloqueios | `/agenda/bloqueios` |
| 29 | agenda-horarios | `/agenda/horarios` |
| 30 | bi-executivo | `/bi` |
| 31 | bi-insights | `/bi?visao=insights` |
| 32 | financeiro-visao | `/financeiro` |
| 33 | titulos-lista | `/financeiro/titulos` |
| 34 | titulos-todos | `/financeiro/titulos?status=todos` |
| 35 | titulos-novo | `/financeiro/titulos/novo` |
| 36 | titulos-detalhe | `/financeiro/titulos/visualizar/3` |
| 37 | contas-lista | `/financeiro/contas` |
| 38 | contas-novo | `/financeiro/contas/novo` |
| 39 | contas-extrato | `/financeiro/contas/extrato/2` |
| 40 | contas-transferir | `/financeiro/contas/transferir` |
| 41 | fluxo-de-caixa | `/financeiro/fluxo` |
| 42 | dre-competencia | `/financeiro/dre?mes=2026-11&regime=competencia` |
| 43 | dre-caixa | `/financeiro/dre?mes=2026-11&regime=caixa` |
| 44 | formas-lista | `/financeiro/formas` |
| 45 | formas-novo | `/financeiro/formas/novo` |
| 46 | tipos-lista | `/financeiro/tipos` |
| 47 | categorias-lista | `/financeiro/categorias` |
| 48 | modo-vendedor | `/vendedor` |
| 49 | rota-inexistente | `/nao-existe` |
| 50 | dashboard-escuro | `/dashboard` |
| 51 | bi-escuro | `/bi` |
| 52 | dashboard-rosa-compacto | `/dashboard` |

## Redirecionamentos observados (permissão e entrada)

- `admin` abre `/vendedor` e vai para `/vendedor#/`
- `vendedor1` abre `/bi` e vai para `/dashboard`
- `vendedor1` abre `/vendedor` e vai para `/vendedor#/`
- `gerente1` abre `/usuarios/permissoes/1` e vai para `/dashboard`
- `salao1` abre `/financeiro/titulos` e vai para `/dashboard`
- `recepcao1` abre `/documentos` e vai para `/dashboard`

## Outros arquivos

- `telas.json`: status, título, URL final e erros de console de cada captura.
- `api/contrato.json`: 60 casos reais de requisição e resposta da API (`/api` e `/vendedor/api`), com o texto exato (`corpo_bruto`).
- `mensagens-antigo.txt`: catálogo das 253 mensagens do código antigo.
- `rea-login.md`: inspeção da tela de login com o REA.

Dados de teste: senhas `Teste@123` dos usuários `vendedor1`, `gerente1`, `salao1`, `recepcao1` valem só no banco descartável `AureaRef`.
