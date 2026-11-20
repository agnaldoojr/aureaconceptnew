-- +goose Up
-- Sessões do navegador guardadas no SQLite (decisão D-12): atualizar ou reiniciar o sistema não derruba os logins.
-- Só o SHA-256 do identificador fica no banco; o identificador de verdade existe apenas no cookie JSESSIONID do navegador.
-- Sessão anônima (ses_usu_codigo nulo) existe só para o formulário de login ter o token CSRF, como o Spring faz.
CREATE TABLE Sessao_Web (
    ses_hash          TEXT PRIMARY KEY CHECK (length(ses_hash) = 64),
    ses_usu_codigo    INTEGER REFERENCES Usuarios (usu_codigo) ON DELETE CASCADE,
    ses_csrf          TEXT NOT NULL,
    ses_criada_em     INTEGER NOT NULL,                                  -- segundos desde 1970 (UTC)
    ses_ultimo_acesso INTEGER NOT NULL,
    ses_duracao_seg   INTEGER NOT NULL CHECK (ses_duracao_seg > 0),     -- inatividade máxima (8 h no navegador; 30 dias no app instalado)
    ses_modo_app      INTEGER NOT NULL DEFAULT 0 CHECK (ses_modo_app IN (0,1)),
    ses_substituida   INTEGER NOT NULL DEFAULT 0 CHECK (ses_substituida IN (0,1)), -- um novo login do mesmo usuário encerra esta
    ses_flash_erro    TEXT,                                              -- mensagem de uma só leitura (vira o "flash" do Spring)
    ses_flash_sucesso TEXT,
    ses_flash_info    TEXT
);
CREATE INDEX IX_Sessao_Web_Usuario ON Sessao_Web (ses_usu_codigo);
CREATE INDEX IX_Sessao_Web_Acesso  ON Sessao_Web (ses_ultimo_acesso);

-- +goose Down
-- Sem retorno: restaure um backup.
SELECT 1;
