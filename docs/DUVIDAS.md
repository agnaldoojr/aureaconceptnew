# Dúvidas, divergências e funcionalidades não documentadas

> Itens que **precisam da sua decisão antes (ou durante) da Fase 2**. Cada um traz o contexto, as opções e a minha recomendação.
> Os marcados **[bloqueia]** impedem uma parte da implementação até você responder; os demais têm um padrão seguro que eu uso se você não disser nada.
> Referências: [INVENTARIO.md](INVENTARIO.md), [PARIDADE.md](PARIDADE.md).

## Respostas do responsável (20/11/2026)

| Dúvida | Resposta | Efeito |
|---|---|---|
| D-15 Go | Instalar com `winget` | **Feito**: Go 1.27.4 em `C:\Program Files\Go` |
| D-02 Visual | **Visual atual publicado** (Inter + Cormorant, claro/escuro, destaques, densidade, menu recolhível, Ctrl+K) | Estáticos copiados sem alteração; comparação visual contra `referencia/` |
| D-01 Senha do admin | **Exigir `ADMIN_SENHA_INICIAL`**; migração recusa concluir se o hash for o do `Admin@123` | Item SEG-03 e MIG-04 |
| D-03 Excluir produto | **Corrigir** (excluir produto sem documentos) | CAD-03 é melhoria intencional |
| D-04 CDN | **Servir localmente** Font Awesome, Chart.js e fontes (mesmas versões, visual igual) | UI-06 |
| D-12 Sessões | **Persistir no SQLite** (atualizar/reiniciar não derruba os logins) | SEG-05/06; contador de login segue em memória |
| D-17 Totais de estoque | **Esconder** de quem não tem a permissão (depois de provada a paridade) | GES-08 |
| D-08 rea.tools / última linha do pedido | **Ainda sem resposta** (não bloqueia o início da Fase 2) | pendente |

Os demais itens (D-05, D-06, D-07, D-09, D-10, D-11, D-13, D-14, D-16) seguem com o padrão descrito em cada um.

## Decisões que preciso de você

### D-01 · Senha padrão do `admin` (`Admin@123`) **[bloqueia SEG-03 e a migração]**
- **O que achei:** `database/schema.sql` cria o `admin` com a senha pública `Admin@123`. O `CLAUDE.md` do projeto antigo diz "trocar em produção", mas **não há registro de que a senha da VPS já foi trocada**.
- **Pedido novo:** não manter senha padrão em produção.
- **Proposta:** o sistema novo **não cria nenhum usuário com senha fixa**. No primeiro boot, sem usuário `admin`, exige `ADMIN_SENHA_INICIAL` (variável obrigatória, mínimo de complexidade; o app recusa subir sem ela) e grava o hash. Na **migração**, o hash do `admin` vem do banco antigo: se ainda for `Admin@123`, a ferramenta de migração **avisa e recusa concluir** até a senha ser trocada (ou até você informar uma nova).
- **Pergunta:** posso fazer assim? E a senha do `admin` na VPS já foi trocada?

### D-02 · Qual visual manter: o do pedido (prata/linho com **Jost**) ou o atual publicado (**Inter** + tema claro/escuro, destaques, densidade, menu recolhível, Ctrl+K)? **[bloqueia UI-04/UI-06]**
- **O que achei:** o texto do pedido cita "Cormorant Garamond + Jost". O sistema **publicado hoje** (redesenho de 19/11/2026) usa **Inter** no corpo e Cormorant só em títulos/logo; **a fonte Jost não aparece em lugar nenhum do código atual**. O visual atual também tem tema escuro, 4 cores de destaque, 3 densidades, menu recolhível e paleta Ctrl+K.
- **Proposta (recomendada):** manter **exatamente o visual atual** (é o que os usuários veem e o que as capturas de referência mostram). Os tokens `--silver*`, `style.css` e o logo são copiados sem alteração.
- **Pergunta:** confirma que "manter o design" significa o visual atual? (Se for o antigo com Jost, preciso do CSS dessa versão.)

### D-03 · Excluir produto falha sempre (bug do sistema antigo)
- **O que achei:** `POST /produtos/excluir/{id}` sempre cai em "Não foi possível excluir o produto…" porque a linha de `Estoque` tem FK e o serviço não a remove (já documentado no `CLAUDE.md`).
- **Opções:** (a) reproduzir o bug (paridade literal); (b) **corrigir**: excluir produto sem nenhum documento (removendo a linha de estoque zerada e as unidades extras); com documentos, manter a mensagem atual.
- **Recomendação:** (b). Muda um comportamento "quebrado", então registro como melhoria intencional na matriz (CAD-03). **Pergunta:** (a) ou (b)?

### D-04 · Dependências de CDN nas telas (Font Awesome 6.5.0, Chart.js 4.4.1, Google Fonts)
- **O que achei:** as telas carregam esses três recursos de CDNs públicas. Isso funciona, mas depende de internet no navegador e envia o IP dos usuários a terceiros.
- **Opções:** (a) manter as CDNs (paridade exata); (b) **servir os mesmos arquivos localmente** (copiados, sem mudar versão nem aparência), dentro do binário.
- **Recomendação:** (b), por privacidade, desempenho e porque o app instalado no iPhone fica menos frágil. O visual não muda. **Pergunta:** (a) ou (b)?

### D-08 · O pedido está incompleto em dois pontos **[bloqueia parte de "ENGENHARIA" e do relatório final]**
1. Na seção **ENGENHARIA** ficou o texto-modelo "`[DESCREVA COMO: ex. levantamento de requisitos, diagramas, etc.]`" sem preenchimento: **como você quer usar o rea.tools** (https://rea.tools/pt-BR/) como apoio e em que formato devem ficar os artefatos em `docs/`?
2. A última linha do pedido está cortada: "*Entregue ao final: relatório de paridade…, consumo de RAM e passo a passo de*" — **passo a passo de quê?** (Imagino "deploy na Hostinger / migração dos dados / troca do endereço no app"; confirme.)
- **Enquanto isso:** uso o REA onde ele agrega (captura de rede/telas do sistema rodando, inspeção do APK) e deixo diagramas em Mermaid dentro de `docs/*.md`.

### D-10 · Backup real da VPS para ensaiar a migração (MIG-06)
- Para provar a migração com dados de verdade preciso, **mais adiante**, de um backup (`.bak`) do banco da VPS (ou de acesso de leitura a ele), a ser fornecido por você. Não vou acessar a VPS. Para desenvolver, uso o banco de referência gerado pelo roteiro `e2e/semear.mjs`.
- **Pergunta (para depois da Fase 2):** como prefere me entregar o `.bak`?

### D-12 · Sessões e limite de login em memória (uma instância)
- **Hoje:** sessões e contadores ficam na memória; **reiniciar derruba todos os logins** (o README avisa). O pedido também manda manter em memória e rodar UMA instância.
- **Opção melhor (opcional):** gravar as sessões no próprio SQLite para sobreviverem a reinícios/atualizações (o `atualizar.sh` deixaria de derrubar todo mundo). Contadores de login continuam em memória.
- **Recomendação:** manter a **paridade (memória)** na Fase 2 e tratar "sessões persistentes" como melhoria opcional depois. **Pergunta:** concorda?

## Pontos em que já tenho um padrão seguro (responda só se discordar)

### D-05 · Arredondamento do total do documento
- `itemdocto_total = qtd × valor` (qtd com 3 casas × valor com 2 → 5 casas) é gravado em coluna de 2 casas; `docto_valor` é a **soma dos produtos sem arredondar**, arredondada **uma vez** ao gravar. Isso pode diferir em centavos da soma dos totais arredondados por item.
- **Padrão:** reproduzir exatamente (cálculo em decimal com 5 casas e arredondamento HALF_UP na gravação, como o SQL Server faz), com testes *golden* contra o antigo (DOC-04). Se o teste achar divergência real, registro aqui antes de decidir.

### D-06 · Formatos de número em mensagens (inconsistentes no antigo)
- A mensagem da baixa mostra "Taxa de R$ **3.99** lançada" (ponto) e a da transferência "R$ **50,00**" (vírgula). **Padrão:** reproduzir cada mensagem como está (paridade textual), anotando a inconsistência para você decidir depois.

### D-07 · Meta de venda "loja toda" (vendedor NULL) e índice único
- No SQL Server, o índice `UNIQUE (meta_mes, meta_ups_vendedor)` trata NULL como valor (só uma meta "loja toda" por mês). No SQLite, NULLs são distintos. **Padrão:** índice único parcial para `vendedor IS NULL`, preservando a regra.

### D-09 · O sistema antigo foi executado sem Docker (informativo)
- O Docker Desktop está desligado neste PC e ligá-lo reiniciaria sozinhas as pilhas `mural-*` do projeto anterior (que o `CLAUDE.md` manda não mexer). Rodei o **mesmo código** (`aurea-concept-1.0.0.jar`, commit `7c3b487`) com Java direto contra um SQL Server local e um banco descartável `AureaRef`, na porta 8091. O comportamento do sistema é o mesmo; só não medi o consumo de RAM **do contêiner** (o Java sozinho ocupou ~330 MB logo na partida). Na Fase 2 repito a comparação de RAM em contêineres, se você ligar o Docker.

### D-11 · Página de erro (confirmado na captura `referencia/admin/desktop/49-rota-inexistente.png`)
- Erros não tratados no antigo mostram a página crua do Tomcat ("HTTP Status 500 – Internal Server Error"). **Padrão:** página simples em português, mesmo status HTTP e sem vazar detalhes (melhoria pequena, registrada em INF-09).

### D-13 · Formato JSON dos números da API
- O Jackson do Spring serializa `BigDecimal` como número JSON. Capturei a resposta **textual** exata em `docs/referencia/api/contrato.json` (`corpo_bruto`) para o Go reproduzir (escala, ordem dos campos, datas ISO).
- **Verificado no código do app** (`ApiClient.kt`): ele lê com `org.json` e converte com `BigDecimal(o.get(chave).toString())`, então tolera `120` ou `120.00`, mas exige **número JSON (não texto)**, campos com os mesmos nomes, datas `aaaa-mm-dd`, `erro` em caso de falha, e ignora campos extras. **Padrão:** reproduzir o texto idêntico do antigo mesmo assim (o teste de contrato compara o `corpo_bruto`).
- O app chama `GET /api/clientes?limite=200&busca=`, `GET /api/produtos?limite=300&estoque=…&busca=`, `GET /api/pedidos?limite=100&busca=`, `GET /api/produtos/{id}/ultima-venda?clienteId=`, `POST /api/pedidos` com `{clienteId, formaPagamentoId, parcelas, itens:[{produtoId, quantidade, valor, unidade?}]}`, e usa timeout de 8 s (conexão) e 20 s (leitura).

### D-14 · O que ainda não li a fundo (honestidade)
- O `BiService` (Painel Executivo **e** Insights) já está lido e descrito em `INVENTARIO.md` (RN-BI). **Ainda não li linha a linha** os templates de tela (HTML/JS dos formulários de documento, agenda e BI, ~3,5 mil linhas) nem o `style.css`; vão ser portados na Fase 2 (UI-07) com a comparação visual e os testes *golden* contra o antigo.

### D-15 · O Go não está instalado neste computador **[bloqueia o início da Fase 2]**
- `go version` não existe aqui (só há Java 21, Maven, Node 26 e o Edge). A Fase 2 precisa do **Go 1.22 ou mais novo**.
- **Proposta:** instalar o Go oficial pelo `winget install --exact --id GoLang.Go` (cerca de 150 MB, instala em `C:\Program Files\Go`; é uma mudança no seu PC, por isso peço seu OK). Alternativa sem instalar: baixar o ZIP oficial para uma pasta dentro do projeto (`.tools/go`, ignorada pelo Git).
- **Pergunta:** posso instalar pelo `winget` na hora de começar a Fase 2?

### D-16 · Navegador para os testes E2E
- O download do Chromium do Playwright **travou na extração** neste PC. Os testes E2E vão usar o **Microsoft Edge já instalado** (`channel: 'msedge'` no Playwright), sem baixar nada. Em outro computador/CI usa-se `npx playwright install chromium`. Informativo; nenhuma ação sua.

### D-17 · O dashboard do antigo mostra totais de estoque a quem não tem permissão de estoque (achado nas capturas)
- **O que achei** (`referencia/recepcao1/desktop/04-documentos-negado.png`, `referencia/vendedor1/...`): o dashboard exibe "Produtos cadastrados", "Quantidade em estoque" e **"Valor total em estoque"** (que é a soma de saldo × **custo médio**) para **qualquer usuário logado**, inclusive a Recepção (que não tem `ESTOQUE`). Também mostra os atalhos "Ver Estoque", "Lucratividade" e "Relatórios" mesmo sem permissão (o clique é negado depois) e o bloco "Últimos Lançamentos" vazio ("Nenhum documento lançado ainda"). O `CLAUDE.md` do antigo já registra um vazamento parecido de custo no `/estoque` para vendedores.
- **Opções:** (a) reproduzir exatamente (paridade); (b) esconder esses totais e atalhos de quem não tem `ESTOQUE`/`RELATORIOS`/`DOCUMENTOS` (melhoria de segurança, pequena mudança visual).
- **Recomendação:** (b), mas só depois da paridade provada; na Fase 2 implemento (a) e deixo (b) como item separado na matriz (GES-08). **Pergunta:** (a) por enquanto e (b) depois, ou (b) já?

### D-18 · Pequenas diferenças deliberadas nas telas de Pessoas e Unidades **[decidido; avise se quiser o contrário]**
- **Data de nascimento inválida** (texto que não é data, só por requisição forjada, pois o navegador usa seletor de data): o antigo mostraria o texto técnico do Spring ("Failed to convert property value…"). O novo mostra **"Data de nascimento inválida"** no mesmo lugar. Nenhum usuário chega nisso pela tela.
- **Menu lateral em Pessoas**: no antigo "Pessoas" **não fica destacado** na lista nem no formulário (o `activePage` era definido num bloco do template que fechava na hora e nunca chegava ao layout). Mantive igual, para a tela ser idêntica; se quiser corrigir, é uma linha.
- **`[` na busca**: no antigo a busca por texto usa `LIKE` do SQL Server, em que `[` abre uma faixa de caracteres (`[a-c]`); no SQLite `[` é um caractere comum. `%` e `_` continuam coringas nos dois. Ninguém depende disso.

### D-19 · Dados de exemplo do `schema.sql` antigo
O `schema.sql` do antigo cria **8 produtos, 8 pessoas, estoque e uma compra de exemplo**. O sistema novo **nasce sem nenhum dado de exemplo**. Para os testes comparados começarem do mesmo ponto, existe `tools/demo` (só desenvolvimento). **Pergunta para a migração (D-10):** o banco da VPS ainda tem esses exemplos ou eles já foram limpos? A ferramenta `cmd/migrar` copia o que estiver lá, então só preciso saber para conferir a contagem.

### D-18 (continuação) · Mais duas diferenças deliberadas, em Produtos
- **Número digitado que não é número** (só por requisição forjada): o antigo mostraria o texto técnico do Spring; o novo mostra "Valor inválido" ao lado do campo.
- **Fator ou preço de unidade extra tão pequeno que arredonda para zero** (menor que 0,000001 ou 0,005): o antigo cairia num erro de banco; o novo diz "O fator de X é pequeno demais" ou "O preço de X deve ser maior que zero".

### D-03 · Excluir produto — **implementado (b)** em 22/11/2026
Com a opção que você escolheu (corrigir): produto **sem documento e sem saldo** é excluído junto com a linha de estoque zerada e as unidades extras; com documento ou saldo, nada é apagado e a mensagem continua a do antigo. O golden marca essa diferença como "decidida".
