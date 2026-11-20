# Inspeção da tela de login do sistema antigo com o REA

Ferramenta: `mcp__rea__inspect_web_page` (REA 6.10.1) sobre o Edge 157 com porta de depuração, no sistema antigo rodando em `http://localhost:8091/login`
(origem permitida: só `http://localhost:8091`). Evidência: `ev_987ffd94ffd6d196803c699f89caf216f081c2bbb3ff94150d14121497f4c57b`. Somente leitura (sem navegar, clicar nem executar JS).

Fatos observados (úteis para a paridade):

| Fato | Valor | Consequência para o sistema novo |
|---|---|---|
| Estáticos com hash do conteúdo no nome | `css/style-4c15b40bf919c3846ddb5a6c990fd559.css`, `js/tema-init-e19592f995fc973379b324397402779e.js`, `img/logo-aurea-232c4b2b82b6d8b0d4466bffd03907eb.webp`, `img/pwa/manifest-396fa70c2fe3bac85ec18633558a6de1.json`, `img/pwa/apple-touch-icon-bd959ad16079dfb2aba0caa50ffbeec9.png` | INF-08: o Go precisa publicar os mesmos arquivos com hash (MD5 de 32 hex do conteúdo) e o HTML apontar para eles |
| Recursos próprios carregados na tela de login | manifesto, `tema-init.js`, `style.css` (65.797 bytes) | tamanho de referência do CSS |
| Recursos de terceiros | 4 `<link>` fora da origem (Google Fonts ×3 e Font Awesome) | DUVIDAS D-04 |
| Console do navegador | sem mensagens de erro | critério E2E: nenhuma tela nova pode gerar erro de console |
| `<html>` com atributos `lang`, `data-theme`, `data-theme-pref`, `data-density` | definidos pelo `tema-init.js` antes da pintura | UI-02 |
| Formulário | `POST /login` com 2 campos ocultos (`_csrf`, `modoApp`) + `login` (obrigatório, foco automático) e `senha` (obrigatória) + botão de mostrar senha | SEG-01, SEG-06, SEG-08 |
| Acessibilidade | rótulos (`label for`) ligados aos campos; `h1` "Áurea Concept"; `h3` "Bem-vindo"; imagem do logo com `alt` | UI-05 |
| Armazenamento no navegador | vazio (nenhuma chave) | só depois do login o app grava `aurea-*` |

Limitação do REA nesta captura: a inspeção parte do momento em que ele se conecta (não vê o carregamento anterior) e não guarda cabeçalhos, cookies nem corpos.
