# ◆ Áurea Concept (novo) — Go + SQLite

Migração do sistema de gestão **Áurea Concept** (loja de roupa + salão + gerência + app Android) de Java/Spring Boot/SQL Server para uma stack leve
(**Go + SQLite**), mantendo o **design** e **todas as funcionalidades**, para rodar em uma VPS pequena (meta: **≤ 512 MB de RAM no total**, app + proxy).

> **Estado atual: Fase 1 (análise) concluída e aprovada; Fase 2 (implementação) em andamento.** O alicerce (regras de dinheiro, banco SQLite com migrations, BCrypt, permissões, configuração por variáveis de ambiente e admin inicial) está pronto e testado contra valores gerados pelo sistema Java; **ainda não há servidor HTTP**. Veja [docs/PROGRESSO.md](docs/PROGRESSO.md).

## Documentação (comece por aqui)
| Arquivo | Conteúdo |
|---|---|
| [docs/INVENTARIO.md](docs/INVENTARIO.md) | Rotas, API, segurança, tabelas, regras de negócio numeradas, telas, pastas auxiliares e comportamentos sutis do sistema antigo |
| [docs/PARIDADE.md](docs/PARIDADE.md) | **Matriz de paridade**: cada funcionalidade → como será testada → status (pendente / feito / testado) |
| [docs/DUVIDAS.md](docs/DUVIDAS.md) | Decisões que dependem de você, divergências e pontos não documentados |
| [docs/DECISOES.md](docs/DECISOES.md) | Stack, dinheiro e arredondamento, SQLite, sessão/segurança, estrutura do repositório, estratégia de testes |
| [docs/referencia/](docs/referencia/) | Capturas de tela do sistema antigo (desktop e celular, por perfil), `telas.json`, contrato real da API (`api/contrato.json`), catálogo de mensagens |

## Ferramentas de desenvolvimento e teste (`e2e/`, só Node, fora da produção)
O sistema **antigo** é o oráculo: roda localmente (somente leitura do repositório `agnaldoojr/aureaconcept`) e as ferramentas abaixo o semeiam e o fotografam.
O mesmo roteiro será executado no sistema novo para comparar.

```powershell
cd e2e
npm install                                   # instala o Playwright (só desenvolvimento)
bash ref-antigo.sh reset                      # recria o banco AureaRef e sobe o sistema antigo em http://localhost:8091
node semear.mjs                               # dados de referência pelas telas e pela API (BASE_URL=... para outro sistema)
node capturar-api.mjs                         # contrato real da API  -> docs/referencia/api/contrato.json
$env:PW_CANAL='msedge'; node capturar-referencia.mjs   # telas -> docs/referencia/<perfil>/<desktop|mobile>/
```
Requisitos para o sistema antigo: JDK 17+, SQL Server local (instância `SQLEXPRESS`, usuário de desenvolvimento) e o jar compilado no clone de referência
(`mvn -o package -DskipTests`). O navegador dos testes é o Edge do Windows (`PW_CANAL=msedge`); em outro computador use `npx playwright install chromium`.
As senhas dos usuários criados pela semeadura são **valores de teste** do banco descartável; nada disso vai para produção.

## Regras do projeto
Ver [CLAUDE.md](CLAUDE.md): este repositório é independente dos projetos anteriores (que ficam somente para leitura), commits e pushes só quando o responsável pedir,
sem segredos no Git, nenhuma limpeza de banco nos comandos.

## Desenvolvimento (Go)
```powershell
$env:Path = "C:\Program Files\Go\bin;" + $env:Path    # Go 1.27 instalado pelo winget
go vet ./... ; go test ./... -count=1
```
Progresso e próximos passos: [docs/PROGRESSO.md](docs/PROGRESSO.md). Os valores de referência do Java (`testdata/golden/`) são gerados por
`java tools/golden/Golden.java testdata/golden/dinheiro.csv` (Java só para desenvolvimento).
