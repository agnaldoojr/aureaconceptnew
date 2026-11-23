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
    ses_substituida   INTEGER NOT NULL DEFAULT 0 CHECK (ses_substituida IN (0,1)) -- um novo login do mesmo usuário encerra esta
);
CREATE INDEX IX_Sessao_Web_Usuario ON Sessao_Web (ses_usu_codigo);
CREATE INDEX IX_Sessao_Web_Acesso  ON Sessao_Web (ses_ultimo_acesso);

-- Mensagens de uma só leitura (o "FlashMap" do Spring): uma FILA por sessão, cada item amarrado ao endereço de destino do
-- redirecionamento e válido por 180 s. Cada visita ao destino consome o item mais antigo que combina, como no antigo.
CREATE TABLE Flash_Web (
    flash_codigo   INTEGER PRIMARY KEY AUTOINCREMENT,
    flash_ses_hash TEXT NOT NULL REFERENCES Sessao_Web (ses_hash) ON DELETE CASCADE,
    flash_destino  TEXT,            -- nulo = ainda não encaminhado (a requisição em curso vai redirecionar)
    flash_expira   INTEGER,
    flash_erro     TEXT,
    flash_sucesso  TEXT,
    flash_info     TEXT
);
CREATE INDEX IX_Flash_Web_Sessao ON Flash_Web (flash_ses_hash, flash_codigo);

-- +goose Down
-- Sem retorno: restaure um backup.
SELECT 1;
