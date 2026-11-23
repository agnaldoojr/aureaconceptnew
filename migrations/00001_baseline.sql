-- +goose Up
-- Estado final do banco do sistema antigo (schema.sql + 02 a 13) convertido para SQLite. Nomes de tabelas e colunas IGUAIS aos do
-- SQL Server (a ferramenta cmd/migrar copia 1:1). Convenções (docs/DECISOES.md T-08, T-12):
--   * dinheiro/preço/total  INTEGER em centavos        (DECIMAL(18,2) x 100)
--   * quantidade            INTEGER em milésimos       (DECIMAL(18,3) x 1.000)
--   * custo médio           INTEGER em décimos de milésimo (DECIMAL(18,4) x 10.000)
--   * fator de conversão    INTEGER em milionésimos    (DECIMAL(18,6) x 1.000.000)
--   * percentuais           INTEGER em centésimos      (DECIMAL(5,2) x 100: 12,50 % = 1250)
--   * DATE 'AAAA-MM-DD'; DATETIME 'AAAA-MM-DD HH:MM:SS' (horário local); TIME 'HH:MM'; BIT 0/1
--   * comparação sem maiúscula e sem acento (collation Latin1_General_CI_AI do SQL Server): COLLATE AUREA_CIAI

CREATE TABLE Segmento (
    seg_codigo INTEGER PRIMARY KEY AUTOINCREMENT,
    seg_nome   TEXT NOT NULL COLLATE AUREA_CIAI,
    seg_ativo  INTEGER NOT NULL DEFAULT 1 CHECK (seg_ativo IN (0,1)),
    CONSTRAINT UQ_Segmento_Nome UNIQUE (seg_nome)
);

CREATE TABLE Unidade_Medida (
    und_codigo    INTEGER PRIMARY KEY AUTOINCREMENT,
    und_sigla     TEXT NOT NULL COLLATE AUREA_CIAI,
    und_descricao TEXT COLLATE AUREA_CIAI NOT NULL,
    und_ativo     INTEGER NOT NULL DEFAULT 1 CHECK (und_ativo IN (0,1)),
    CONSTRAINT UQ_Unidade_Medida_Sigla UNIQUE (und_sigla)
);

CREATE TABLE Tipo_Titulo (
    tpt_codigo   INTEGER PRIMARY KEY AUTOINCREMENT,
    tpt_nome     TEXT COLLATE AUREA_CIAI NOT NULL,
    tpt_natureza TEXT NOT NULL CHECK (tpt_natureza IN ('RECEBER','PAGAR')),
    tpt_ativo    INTEGER NOT NULL DEFAULT 1 CHECK (tpt_ativo IN (0,1))
);

CREATE TABLE Conta_Financeira (
    cfn_codigo        INTEGER PRIMARY KEY AUTOINCREMENT,
    cfn_nome          TEXT COLLATE AUREA_CIAI NOT NULL,
    cfn_tipo          TEXT NOT NULL CHECK (cfn_tipo IN ('CAIXA','BANCO','MAQUININHA','OUTRA')),
    cfn_saldo_inicial INTEGER NOT NULL DEFAULT 0,
    cfn_ativo         INTEGER NOT NULL DEFAULT 1 CHECK (cfn_ativo IN (0,1)),
    cfn_criado_em     TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);

CREATE TABLE Categoria_Financeira (
    cat_codigo         INTEGER PRIMARY KEY AUTOINCREMENT,
    cat_nome           TEXT COLLATE AUREA_CIAI NOT NULL,
    cat_tipo           TEXT NOT NULL CHECK (cat_tipo IN ('RECEITA','DESPESA')),
    cat_grupo_dre      TEXT NOT NULL CHECK (cat_grupo_dre IN
                         ('RECEITA_BRUTA','DEDUCOES','DESPESA_OPERACIONAL','OUTRAS_RECEITAS','OUTRAS_DESPESAS','NAO_DRE')),
    cat_codigo_sistema TEXT COLLATE AUREA_CIAI,
    cat_ativo          INTEGER NOT NULL DEFAULT 1 CHECK (cat_ativo IN (0,1))
);
CREATE UNIQUE INDEX UX_Categoria_CodigoSistema ON Categoria_Financeira (cat_codigo_sistema) WHERE cat_codigo_sistema IS NOT NULL;

CREATE TABLE Forma_Pagamento (
    fpg_codigo             INTEGER PRIMARY KEY AUTOINCREMENT,
    fpg_nome               TEXT COLLATE AUREA_CIAI NOT NULL,
    fpg_tpt_codigo         INTEGER NOT NULL REFERENCES Tipo_Titulo (tpt_codigo),
    fpg_permite_parcelas   INTEGER NOT NULL DEFAULT 0 CHECK (fpg_permite_parcelas IN (0,1)),
    fpg_max_parcelas       INTEGER NOT NULL DEFAULT 1,
    fpg_taxa_percentual    INTEGER NOT NULL DEFAULT 0,          -- centésimos de %
    fpg_dias_vencimento    INTEGER NOT NULL DEFAULT 0,
    fpg_intervalo_parcelas INTEGER NOT NULL DEFAULT 30,
    fpg_baixa_automatica   INTEGER NOT NULL DEFAULT 0 CHECK (fpg_baixa_automatica IN (0,1)),
    fpg_cfn_codigo         INTEGER REFERENCES Conta_Financeira (cfn_codigo),
    fpg_nome_baixa         TEXT COLLATE AUREA_CIAI,
    fpg_ativo              INTEGER NOT NULL DEFAULT 1 CHECK (fpg_ativo IN (0,1))
);

CREATE TABLE Pessoas (
    ups_codigo      INTEGER PRIMARY KEY AUTOINCREMENT,
    ups_nome        TEXT COLLATE AUREA_CIAI NOT NULL,
    ups_cpf         TEXT COLLATE AUREA_CIAI,
    ups_telefone    TEXT COLLATE AUREA_CIAI,
    ups_cliente     INTEGER NOT NULL DEFAULT 0 CHECK (ups_cliente IN (0,1)),
    ups_fornecedor  INTEGER NOT NULL DEFAULT 0 CHECK (ups_fornecedor IN (0,1)),
    ups_vendedor    INTEGER NOT NULL DEFAULT 0 CHECK (ups_vendedor IN (0,1)),
    ups_funcionario INTEGER NOT NULL DEFAULT 0 CHECK (ups_funcionario IN (0,1)),
    ups_ativo       INTEGER NOT NULL DEFAULT 1 CHECK (ups_ativo IN (0,1)),
    ups_criado_em   TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    ups_nascimento  TEXT
);
CREATE INDEX IX_Pessoas_Nome ON Pessoas (ups_nome COLLATE AUREA_CIAI);
CREATE INDEX IX_Pessoas_CPF  ON Pessoas (ups_cpf);

CREATE TABLE Pessoa_Segmento (
    ps_ups_codigo INTEGER NOT NULL REFERENCES Pessoas (ups_codigo),
    ps_seg_codigo INTEGER NOT NULL REFERENCES Segmento (seg_codigo),
    CONSTRAINT PK_Pessoa_Segmento PRIMARY KEY (ps_ups_codigo, ps_seg_codigo)
) WITHOUT ROWID;
CREATE INDEX IX_Pessoa_Segmento_Seg ON Pessoa_Segmento (ps_seg_codigo);

CREATE TABLE Usuarios (
    usu_codigo       INTEGER PRIMARY KEY AUTOINCREMENT,
    usu_nome         TEXT COLLATE AUREA_CIAI NOT NULL,
    usu_login        TEXT NOT NULL COLLATE AUREA_CIAI,
    usu_senha        TEXT NOT NULL,                              -- BCrypt ($2a$10$...)
    usu_ativo        INTEGER NOT NULL DEFAULT 1 CHECK (usu_ativo IN (0,1)),
    usu_criado_em    TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    usu_ups_vendedor INTEGER REFERENCES Pessoas (ups_codigo),
    CONSTRAINT UQ_Usuarios_Login UNIQUE (usu_login)
);

CREATE TABLE Direito_Usuario (
    dusu_codigo      INTEGER PRIMARY KEY AUTOINCREMENT,
    dusu_usu_codigo  INTEGER NOT NULL REFERENCES Usuarios (usu_codigo) ON DELETE CASCADE,
    dusu_descricao   TEXT NOT NULL,                              -- módulo (ex.: PRODUTOS)
    dusu_inclusao    INTEGER NOT NULL DEFAULT 0 CHECK (dusu_inclusao IN (0,1)),
    dusu_exclusao    INTEGER NOT NULL DEFAULT 0 CHECK (dusu_exclusao IN (0,1)),
    dusu_consulta    INTEGER NOT NULL DEFAULT 0 CHECK (dusu_consulta IN (0,1)),
    dusu_atualizacao INTEGER NOT NULL DEFAULT 0 CHECK (dusu_atualizacao IN (0,1))
);
CREATE INDEX IX_Direito_Usuario ON Direito_Usuario (dusu_usu_codigo);

CREATE TABLE Token_App (
    tok_codigo      INTEGER PRIMARY KEY AUTOINCREMENT,
    tok_usu_codigo  INTEGER NOT NULL REFERENCES Usuarios (usu_codigo) ON DELETE CASCADE,
    tok_hash        TEXT NOT NULL CHECK (length(tok_hash) = 64), -- SHA-256 em hexadecimal
    tok_dispositivo TEXT COLLATE AUREA_CIAI,
    tok_criado_em   TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    tok_expira_em   TEXT NOT NULL,
    tok_ultimo_uso  TEXT,
    tok_revogado    INTEGER NOT NULL DEFAULT 0 CHECK (tok_revogado IN (0,1)),
    CONSTRAINT UQ_TokenApp_Hash UNIQUE (tok_hash)
);

CREATE TABLE Produto (
    upr_codigo        INTEGER PRIMARY KEY AUTOINCREMENT,
    upr_nome          TEXT COLLATE AUREA_CIAI NOT NULL,
    upr_complemento   TEXT COLLATE AUREA_CIAI,
    upr_und           TEXT COLLATE AUREA_CIAI NOT NULL,
    upr_preco         INTEGER NOT NULL CHECK (upr_preco >= 0),   -- centavos (a regra "preço >= 0,01" é da aplicação)
    upr_ativo         INTEGER NOT NULL DEFAULT 1 CHECK (upr_ativo IN (0,1)),
    upr_criado_em     TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    upr_cat_venda     INTEGER REFERENCES Categoria_Financeira (cat_codigo),
    upr_cat_compra    INTEGER REFERENCES Categoria_Financeira (cat_codigo),
    upr_seg_codigo    INTEGER REFERENCES Segmento (seg_codigo),
    upr_servico       INTEGER NOT NULL DEFAULT 0 CHECK (upr_servico IN (0,1)),
    upr_comissao_perc INTEGER,                                    -- centésimos de %
    upr_duracao_min   INTEGER,
    CONSTRAINT CK_Produto_Comissao CHECK (upr_comissao_perc IS NULL OR (upr_comissao_perc >= 0 AND upr_comissao_perc <= 10000)),
    CONSTRAINT CK_Produto_Duracao  CHECK (upr_duracao_min IS NULL OR (upr_duracao_min > 0 AND upr_duracao_min <= 1440))
);
CREATE INDEX IX_Produto_Nome ON Produto (upr_nome COLLATE AUREA_CIAI);

CREATE TABLE Estoque (
    est_upr_codigo  INTEGER PRIMARY KEY REFERENCES Produto (upr_codigo),
    est_qtd         INTEGER NOT NULL DEFAULT 0,                  -- milésimos
    est_valor_custo INTEGER NOT NULL DEFAULT 0                   -- décimos de milésimo
);

CREATE TABLE Produto_Unidade (
    pun_codigo     INTEGER PRIMARY KEY AUTOINCREMENT,
    pun_upr_codigo INTEGER NOT NULL REFERENCES Produto (upr_codigo),
    pun_sigla      TEXT NOT NULL COLLATE AUREA_CIAI,
    pun_fator      INTEGER NOT NULL CHECK (pun_fator > 0),       -- milionésimos
    pun_preco      INTEGER CHECK (pun_preco IS NULL OR pun_preco > 0),
    CONSTRAINT UQ_Produto_Unidade UNIQUE (pun_upr_codigo, pun_sigla)
);

CREATE TABLE Documento (
    docto_codigo        INTEGER PRIMARY KEY AUTOINCREMENT,
    docto_tipo          TEXT NOT NULL CHECK (docto_tipo IN ('COMPRA','VENDA','ENTRADA','SAIDA')),
    docto_data          TEXT NOT NULL,
    docto_valor         INTEGER NOT NULL DEFAULT 0,
    docto_ups_codigo    INTEGER NOT NULL REFERENCES Pessoas (ups_codigo),
    docto_usu_codigo    INTEGER NOT NULL REFERENCES Usuarios (usu_codigo),
    docto_criado_em     TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    docto_classificacao TEXT COLLATE AUREA_CIAI,
    docto_vendedor      INTEGER REFERENCES Pessoas (ups_codigo),
    docto_seg_codigo    INTEGER REFERENCES Segmento (seg_codigo)
);
CREATE INDEX IX_Documento_Data     ON Documento (docto_data);
CREATE INDEX IX_Documento_Tipo     ON Documento (docto_tipo);
CREATE INDEX IX_Documento_Vendedor ON Documento (docto_vendedor);

CREATE TABLE Itens_Documento (
    itemdocto_codigo        INTEGER PRIMARY KEY AUTOINCREMENT,
    itemdocto_docto_codigo  INTEGER NOT NULL REFERENCES Documento (docto_codigo),
    itemdocto_upr_codigo    INTEGER NOT NULL REFERENCES Produto (upr_codigo),
    itemdocto_upr_und       TEXT COLLATE AUREA_CIAI NOT NULL,
    itemdocto_qtd           INTEGER NOT NULL,                    -- milésimos
    itemdocto_valor         INTEGER NOT NULL,                    -- centavos
    itemdocto_total         INTEGER NOT NULL DEFAULT 0,          -- centavos
    itemdocto_fator         INTEGER NOT NULL DEFAULT 1000000,    -- milionésimos (1,000000)
    itemdocto_prof_codigo   INTEGER REFERENCES Pessoas (ups_codigo),
    itemdocto_comissao_perc INTEGER                              -- centésimos de %
);
CREATE INDEX IX_Itens_Documento_Docto        ON Itens_Documento (itemdocto_docto_codigo);
CREATE INDEX IX_Itens_Documento_Produto      ON Itens_Documento (itemdocto_upr_codigo);
CREATE INDEX IX_Itens_Documento_Profissional ON Itens_Documento (itemdocto_prof_codigo);

CREATE TABLE Titulo (
    tit_codigo         INTEGER PRIMARY KEY AUTOINCREMENT,
    tit_natureza       TEXT NOT NULL CHECK (tit_natureza IN ('RECEBER','PAGAR')),
    tit_origem         TEXT NOT NULL CHECK (tit_origem IN ('DOCUMENTO','AVULSO','TAXA')),
    tit_docto_codigo   INTEGER REFERENCES Documento (docto_codigo),
    tit_ups_codigo     INTEGER REFERENCES Pessoas (ups_codigo),
    tit_cat_codigo     INTEGER NOT NULL REFERENCES Categoria_Financeira (cat_codigo),
    tit_fpg_codigo     INTEGER REFERENCES Forma_Pagamento (fpg_codigo),
    tit_tpt_codigo     INTEGER REFERENCES Tipo_Titulo (tpt_codigo),
    tit_descricao      TEXT COLLATE AUREA_CIAI NOT NULL,
    tit_parcela        INTEGER NOT NULL DEFAULT 1,
    tit_total_parcelas INTEGER NOT NULL DEFAULT 1,
    tit_emissao        TEXT NOT NULL,
    tit_competencia    TEXT NOT NULL,                            -- sempre o dia 1 do mês
    tit_vencimento     TEXT NOT NULL,
    tit_valor          INTEGER NOT NULL,
    tit_valor_baixado  INTEGER NOT NULL DEFAULT 0,
    tit_status         TEXT NOT NULL DEFAULT 'ABERTO' CHECK (tit_status IN ('ABERTO','PARCIAL','BAIXADO','CANCELADO')),
    tit_baixa_origem   INTEGER,                                  -- título de taxa: baixa que o gerou
    tit_usu_codigo     INTEGER REFERENCES Usuarios (usu_codigo),
    tit_criado_em      TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    tit_seg_codigo     INTEGER REFERENCES Segmento (seg_codigo)
);
CREATE INDEX IX_Titulo_Vencimento  ON Titulo (tit_vencimento);
CREATE INDEX IX_Titulo_Competencia ON Titulo (tit_competencia);
CREATE INDEX IX_Titulo_Documento   ON Titulo (tit_docto_codigo);
CREATE INDEX IX_Titulo_Segmento    ON Titulo (tit_seg_codigo);

CREATE TABLE Baixa (
    bx_codigo       INTEGER PRIMARY KEY AUTOINCREMENT,
    bx_tit_codigo   INTEGER NOT NULL REFERENCES Titulo (tit_codigo),
    bx_data         TEXT NOT NULL,
    bx_cfn_codigo   INTEGER NOT NULL REFERENCES Conta_Financeira (cfn_codigo),
    bx_fpg_codigo   INTEGER REFERENCES Forma_Pagamento (fpg_codigo),
    bx_valor        INTEGER NOT NULL,
    bx_juros        INTEGER NOT NULL DEFAULT 0,
    bx_multa        INTEGER NOT NULL DEFAULT 0,
    bx_desconto     INTEGER NOT NULL DEFAULT 0,
    bx_taxa         INTEGER NOT NULL DEFAULT 0,
    bx_observacao   TEXT COLLATE AUREA_CIAI,
    bx_automatica   INTEGER NOT NULL DEFAULT 0 CHECK (bx_automatica IN (0,1)),
    bx_estornada    INTEGER NOT NULL DEFAULT 0 CHECK (bx_estornada IN (0,1)),
    bx_estornada_em TEXT,
    bx_usu_codigo   INTEGER REFERENCES Usuarios (usu_codigo),
    bx_criado_em    TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);
CREATE INDEX IX_Baixa_Data   ON Baixa (bx_data);
CREATE INDEX IX_Baixa_Titulo ON Baixa (bx_tit_codigo);

CREATE TABLE Transferencia_Conta (
    trf_codigo       INTEGER PRIMARY KEY AUTOINCREMENT,
    trf_data         TEXT NOT NULL,
    trf_cfn_origem   INTEGER NOT NULL REFERENCES Conta_Financeira (cfn_codigo),
    trf_cfn_destino  INTEGER NOT NULL REFERENCES Conta_Financeira (cfn_codigo),
    trf_valor        INTEGER NOT NULL CHECK (trf_valor > 0),
    trf_observacao   TEXT COLLATE AUREA_CIAI,
    trf_usu_codigo   INTEGER,
    trf_estornada    INTEGER NOT NULL DEFAULT 0 CHECK (trf_estornada IN (0,1)),
    trf_estornada_em TEXT,
    trf_criado_em    TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    CONSTRAINT CK_Transferencia_Contas CHECK (trf_cfn_origem <> trf_cfn_destino)
);
CREATE INDEX IX_Transferencia_Data ON Transferencia_Conta (trf_data);

CREATE TABLE Movimento_Conta (
    mov_codigo        INTEGER PRIMARY KEY AUTOINCREMENT,
    mov_cfn_codigo    INTEGER NOT NULL REFERENCES Conta_Financeira (cfn_codigo),
    mov_data          TEXT NOT NULL,
    mov_tipo          TEXT NOT NULL CHECK (mov_tipo IN ('ENTRADA','SAIDA')),
    mov_valor         INTEGER NOT NULL,
    mov_descricao     TEXT COLLATE AUREA_CIAI NOT NULL,
    mov_bx_codigo     INTEGER REFERENCES Baixa (bx_codigo),
    mov_conciliado    INTEGER NOT NULL DEFAULT 0 CHECK (mov_conciliado IN (0,1)),
    mov_conciliado_em TEXT,
    mov_criado_em     TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    mov_trf_codigo    INTEGER REFERENCES Transferencia_Conta (trf_codigo)
);
CREATE INDEX IX_Movimento_ContaData     ON Movimento_Conta (mov_cfn_codigo, mov_data);
CREATE INDEX IX_Movimento_Transferencia ON Movimento_Conta (mov_trf_codigo);

CREATE TABLE Meta_Venda (
    meta_codigo       INTEGER PRIMARY KEY AUTOINCREMENT,
    meta_mes          TEXT NOT NULL CHECK (substr(meta_mes, 9, 2) = '01'),
    meta_ups_vendedor INTEGER REFERENCES Pessoas (ups_codigo),   -- nulo = loja toda
    meta_valor        INTEGER NOT NULL CHECK (meta_valor > 0),
    meta_criado_em    TEXT NOT NULL DEFAULT (datetime('now','localtime'))
);
-- no SQL Server o índice único tratava NULL como valor: só uma meta "loja toda" por mês (docs/DUVIDAS.md D-07)
CREATE UNIQUE INDEX UX_Meta_Venda_Mes_Vendedor ON Meta_Venda (meta_mes, meta_ups_vendedor) WHERE meta_ups_vendedor IS NOT NULL;
CREATE UNIQUE INDEX UX_Meta_Venda_Mes_Loja     ON Meta_Venda (meta_mes) WHERE meta_ups_vendedor IS NULL;

CREATE TABLE Agendamento (
    agd_codigo           INTEGER PRIMARY KEY AUTOINCREMENT,
    agd_cliente          INTEGER REFERENCES Pessoas (ups_codigo),
    agd_cliente_nome     TEXT COLLATE AUREA_CIAI,
    agd_cliente_telefone TEXT COLLATE AUREA_CIAI,
    agd_prof_codigo      INTEGER NOT NULL REFERENCES Pessoas (ups_codigo),
    agd_inicio           TEXT NOT NULL,
    agd_fim              TEXT NOT NULL,
    agd_status           TEXT NOT NULL DEFAULT 'AGENDADO'
                         CHECK (agd_status IN ('AGENDADO','CONFIRMADO','EM_ATENDIMENTO','CONCLUIDO','FALTOU','CANCELADO')),
    agd_obs              TEXT COLLATE AUREA_CIAI,
    agd_docto_codigo     INTEGER REFERENCES Documento (docto_codigo) ON DELETE SET NULL,
    agd_usu_codigo       INTEGER REFERENCES Usuarios (usu_codigo),
    agd_criado_em        TEXT NOT NULL DEFAULT (datetime('now','localtime')),
    CONSTRAINT CK_Agendamento_Periodo CHECK (agd_fim > agd_inicio),
    CONSTRAINT CK_Agendamento_Cliente CHECK (agd_cliente IS NOT NULL OR length(coalesce(agd_cliente_nome, '')) > 0)
);
CREATE INDEX IX_Agendamento_Prof_Inicio ON Agendamento (agd_prof_codigo, agd_inicio);
CREATE INDEX IX_Agendamento_Inicio      ON Agendamento (agd_inicio);

CREATE TABLE Agendamento_Servico (
    ags_codigo      INTEGER PRIMARY KEY AUTOINCREMENT,
    ags_agd_codigo  INTEGER NOT NULL REFERENCES Agendamento (agd_codigo) ON DELETE CASCADE,
    ags_upr_codigo  INTEGER NOT NULL REFERENCES Produto (upr_codigo),
    ags_duracao_min INTEGER NOT NULL CHECK (ags_duracao_min > 0),
    ags_valor       INTEGER NOT NULL CHECK (ags_valor >= 0)
);
CREATE INDEX IX_Agd_Servico_Agendamento ON Agendamento_Servico (ags_agd_codigo);

CREATE TABLE Agenda_Horario (
    hor_codigo       INTEGER PRIMARY KEY AUTOINCREMENT,
    hor_prof_codigo  INTEGER NOT NULL REFERENCES Pessoas (ups_codigo),
    hor_dia_semana   INTEGER NOT NULL CHECK (hor_dia_semana BETWEEN 1 AND 7),
    hor_inicio       TEXT NOT NULL,
    hor_fim          TEXT NOT NULL,
    hor_pausa_inicio TEXT,
    hor_pausa_fim    TEXT,
    CONSTRAINT CK_Agenda_Horario_Periodo CHECK (hor_fim > hor_inicio),
    CONSTRAINT CK_Agenda_Horario_Pausa CHECK (
        (hor_pausa_inicio IS NULL AND hor_pausa_fim IS NULL) OR
        (hor_pausa_inicio IS NOT NULL AND hor_pausa_fim IS NOT NULL AND hor_pausa_fim > hor_pausa_inicio))
);
CREATE UNIQUE INDEX UX_Agenda_Horario_Prof_Dia ON Agenda_Horario (hor_prof_codigo, hor_dia_semana);

CREATE TABLE Agenda_Bloqueio (
    blq_codigo      INTEGER PRIMARY KEY AUTOINCREMENT,
    blq_prof_codigo INTEGER REFERENCES Pessoas (ups_codigo),     -- nulo = salão todo
    blq_inicio      TEXT NOT NULL,
    blq_fim         TEXT NOT NULL,
    blq_motivo      TEXT COLLATE AUREA_CIAI,
    CONSTRAINT CK_Agenda_Bloqueio_Periodo CHECK (blq_fim > blq_inicio)
);
CREATE INDEX IX_Agenda_Bloqueio_Inicio ON Agenda_Bloqueio (blq_inicio);

-- ===================== dados iniciais (iguais aos do sistema antigo, na mesma ordem de códigos) =====================
-- A ferramenta cmd/migrar esvazia estas tabelas antes de importar os dados reais (ela preserva os códigos de origem).
INSERT INTO Segmento (seg_nome) VALUES ('Loja de roupa'), ('Salão');

INSERT INTO Unidade_Medida (und_sigla, und_descricao) VALUES
    ('UN','Unidade'), ('CJ','Conjunto'), ('PR','Par'), ('KG','Quilograma'), ('MT','Metro');

INSERT INTO Tipo_Titulo (tpt_nome, tpt_natureza) VALUES
    ('Dinheiro','RECEBER'), ('PIX','RECEBER'), ('Cartão de débito','RECEBER'), ('Cartão de crédito','RECEBER'),
    ('Boleto','RECEBER'), ('Crediário','RECEBER'),
    ('Dinheiro','PAGAR'), ('PIX / transferência','PAGAR'), ('Boleto','PAGAR');

INSERT INTO Conta_Financeira (cfn_nome, cfn_tipo, cfn_saldo_inicial) VALUES
    ('Caixa','CAIXA',0), ('Banco','BANCO',0), ('Maquininha','MAQUININHA',0);

INSERT INTO Categoria_Financeira (cat_nome, cat_tipo, cat_grupo_dre, cat_codigo_sistema) VALUES
    ('Venda de produtos',      'RECEITA','RECEITA_BRUTA',      'VENDA_PRODUTOS'),
    ('Outras receitas',        'RECEITA','OUTRAS_RECEITAS',    NULL),
    ('Taxas de cartão',        'DESPESA','DEDUCOES',           'TAXA_CARTAO'),
    ('Impostos sobre vendas',  'DESPESA','DEDUCOES',           NULL),
    ('Compra de mercadoria',   'DESPESA','NAO_DRE',            'COMPRA_MERCADORIA'),
    ('Aluguel',                'DESPESA','DESPESA_OPERACIONAL',NULL),
    ('Água, luz e internet',   'DESPESA','DESPESA_OPERACIONAL',NULL),
    ('Marketing e divulgação', 'DESPESA','DESPESA_OPERACIONAL',NULL),
    ('Embalagens',             'DESPESA','DESPESA_OPERACIONAL',NULL),
    ('Frete e entregas',       'DESPESA','DESPESA_OPERACIONAL',NULL),
    ('Salários e pró-labore',  'DESPESA','DESPESA_OPERACIONAL',NULL),
    ('Outras despesas',        'DESPESA','OUTRAS_DESPESAS',    NULL);

-- Formas de recebimento (taxa em centésimos de %: 1,99 % = 199) e de pagamento
INSERT INTO Forma_Pagamento (fpg_nome, fpg_tpt_codigo, fpg_permite_parcelas, fpg_max_parcelas, fpg_taxa_percentual,
                             fpg_dias_vencimento, fpg_intervalo_parcelas, fpg_baixa_automatica, fpg_cfn_codigo, fpg_nome_baixa) VALUES
    ('Dinheiro',          (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='Dinheiro'          AND tpt_natureza='RECEBER'), 0, 1,    0,  0, 30, 1, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Caixa'),      'Recebimento em dinheiro'),
    ('PIX',               (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='PIX'               AND tpt_natureza='RECEBER'), 0, 1,    0,  0, 30, 1, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Banco'),      'Recebimento via PIX'),
    ('Cartão de débito',  (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='Cartão de débito'  AND tpt_natureza='RECEBER'), 0, 1,  199,  1, 30, 0, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Maquininha'), 'Recebimento de cartão'),
    ('Cartão de crédito', (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='Cartão de crédito' AND tpt_natureza='RECEBER'), 1, 12, 399, 30, 30, 0, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Maquininha'), 'Recebimento de cartão'),
    ('Boleto',            (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='Boleto'            AND tpt_natureza='RECEBER'), 1, 6,    0, 30, 30, 0, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Banco'),      'Recebimento de boleto'),
    ('Crediário',         (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='Crediário'         AND tpt_natureza='RECEBER'), 1, 6,    0, 30, 30, 0, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Caixa'),      'Recebimento de crediário'),
    ('Dinheiro (pagamento)', (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='Dinheiro'       AND tpt_natureza='PAGAR'),   0, 1,    0,  0, 30, 1, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Caixa'),      'Pagamento em dinheiro'),
    ('PIX / transferência',  (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='PIX / transferência' AND tpt_natureza='PAGAR'), 0, 1,  0,  0, 30, 1, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Banco'),      'Pagamento via PIX'),
    ('Boleto a pagar',       (SELECT tpt_codigo FROM Tipo_Titulo WHERE tpt_nome='Boleto'         AND tpt_natureza='PAGAR'),   1, 6,    0, 30, 30, 0, (SELECT cfn_codigo FROM Conta_Financeira WHERE cfn_nome='Banco'),      'Pagamento de boleto');

-- +goose Down
-- Sem retorno: o banco novo não desfaz a migration de base (restaure um backup).
SELECT 1;
