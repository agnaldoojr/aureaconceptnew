# Aurea Concept (novo) — guia do projeto

Repositório novo, **independente** do projeto anterior "Áurea Concept" (pasta `C:\Users\agnal\aureaconcept`,
GitHub `agnaldoojr/aureaconcept`, publicado na VPS em `https://auraconcept1.store`).
Este projeto: `C:\Users\agnal\aureaconceptnew`, GitHub `https://github.com/agnaldoojr/aureaconceptnew` (criado vazio em 20/11/2026).

O usuário (Agnaldo) fala português (pt-BR): responda e escreva mensagens de commit, mensagens de tela e documentação em português.

## Regra principal: não mexer nos projetos anteriores
- Tudo o que for feito aqui fica **somente** nesta pasta e neste repositório.
- Não editar, commitar, dar push, fazer deploy nem rodar scripts de banco no `aureaconcept`, no `mural-semijoias` ou na VPS
  (`/opt/aurea`, banco `AureaConcept`). Se for preciso consultar algo do projeto anterior, **só ler**.
- Banco de dados, contêineres Docker e portas deste projeto devem ter nomes próprios (nada de reusar `AureaConcept`, `aureaconcept` ou as portas 8088/8090).

## Regras de trabalho combinadas com o usuário
- **Só commitar quando ele pedir; só dar push quando ele pedir.** Mensagens de commit em português, sem acento nos títulos curtos,
  terminando com a linha `Co-Authored-By` do ambiente.
- Nunca versionar segredos (`.env`, chaves, `*.jks`, `keystore.properties`, backups `.bak`).
- Ações que mudam estado fora do projeto (apagar dados, aplicar script em banco real, publicar) pedem confirmação antes.
- Nunca incluir limpeza de banco nos comandos que passar a ele.
- Explicar de forma didática: o usuário está aprendendo Docker, Linux e ambiente web.
- Testar de verdade e dizer com clareza o que **não** foi testado.

## Missão (parte 3)
Migrar o Áurea Concept de Java/Spring Boot/SQL Server para **Go + SQLite**, mantendo o design e todas as funcionalidades, para rodar em VPS ≤ 512 MB (app + Caddy).
Oráculo: o sistema antigo (clone somente leitura em `C:\Users\agnal\aureaconcept-ref`, repositório `agnaldoojr/aureaconcept` no commit `7c3b487`), rodando com
`e2e/ref-antigo.sh reset` (porta 8091, banco descartável `AureaRef`). Documentos em `docs/` (INVENTARIO, PARIDADE, DUVIDAS, DECISOES).

## Estado
- **Fase 1 (análise) concluída e matriz de paridade APROVADA em 20/11/2026** (`docs/PARIDADE.md`, 144 itens); respostas em `docs/DUVIDAS.md`. **Fase 2 em andamento** (progresso em `docs/PROGRESSO.md`).
- Go 1.27.4 instalado via winget (20/11/2026). Testes E2E usam o Edge do Windows (`PW_CANAL=msedge`).
- Regra do pedido: nada é "migrado" sem teste passando e item marcado na matriz; ambiguidades vão para `docs/DUVIDAS.md` e se pergunta antes de decidir.
