package web

import (
	"net/url"
	"strings"
	"testing"

	"github.com/agnaldoojr/aureaconceptnew/internal/auth"
)

func usuarioForm(nome, login string, extra ...string) url.Values {
	v := url.Values{"usuNome": {nome}, "usuLogin": {login}, "novaSenha": {"Senha-Boa-123"}, "usuAtivo": {"true"}, "_usuAtivo": {"on"}, "vendedorId": {""}, "perfil": {""}}
	for i := 0; i+1 < len(extra); i += 2 {
		v.Set(extra[i], extra[i+1])
	}
	return v
}

// direitosDe devolve "MODULO:CIAE" dos direitos do usuário (letras = ações liberadas), só dos módulos com algum direito.
func (a *ambiente) direitosDe(login string) map[string]string {
	a.t.Helper()
	rows, err := a.banco.R.Query(`SELECT d.dusu_descricao, d.dusu_consulta, d.dusu_inclusao, d.dusu_atualizacao, d.dusu_exclusao FROM Direito_Usuario d JOIN Usuarios u ON u.usu_codigo = d.dusu_usu_codigo WHERE u.usu_login = ?`, login)
	if err != nil {
		a.t.Fatal(err)
	}
	defer rows.Close()
	res := map[string]string{}
	n := 0
	for rows.Next() {
		var m string
		var c, i, at, e int
		rows.Scan(&m, &c, &i, &at, &e)
		n++
		s := ""
		for _, p := range []struct {
			v int
			l string
		}{{c, "C"}, {i, "I"}, {at, "A"}, {e, "E"}} {
			if p.v == 1 {
				s += p.l
			}
		}
		if s != "" {
			res[m] = s
		}
	}
	res["#linhas"] = strings.Repeat("x", n)
	return res
}

func TestUsuarioNovoNasceComDireitosPadraoESenhaBCrypt(t *testing.T) {
	a, n := logadoAdmin(t)
	st, loc := n.postar("/usuarios/salvar", usuarioForm("Maria Souza", "maria"))
	if st != 302 || loc != "/usuarios" {
		t.Fatalf("salvar = %d → %s", st, loc)
	}
	n.esperaMensagem("/usuarios", "Usuário salvo com sucesso!")
	d := a.direitosDe("maria")
	if len(d["#linhas"]) != 21 {
		t.Fatalf("esperadas 21 linhas de direito (uma por módulo), vieram %d", len(d["#linhas"]))
	}
	for _, m := range []string{"PRODUTOS", "PESSOAS", "DOCUMENTOS", "ESTOQUE", "RELATORIOS"} {
		if d[m] != "C" {
			t.Errorf("%s deveria nascer só com Consulta, veio %q", m, d[m])
		}
	}
	if len(d) != 6 { // as 5 de Consulta + a chave auxiliar
		t.Errorf("os demais módulos devem nascer zerados: %v", d)
	}
	var hash string
	a.banco.R.QueryRow(`SELECT usu_senha FROM Usuarios WHERE usu_login = 'maria'`).Scan(&hash)
	if !strings.HasPrefix(hash, "$2a$10$") && !strings.HasPrefix(hash, "$2b$10$") || !auth.ConferirSenha(hash, "Senha-Boa-123") {
		t.Errorf("a senha deveria estar em BCrypt fator 10: %q", hash)
	}
	// o usuário novo entra de verdade, com o login em outra caixa e sem acento
	m := a.novoNavegador()
	m.entrar("MARIA", "Senha-Boa-123")
	if !strings.Contains(m.paginaOK("/dashboard"), `href="/produtos"`) {
		t.Error("o usuário novo deveria ver Produtos (Consulta padrão)")
	}
}

func TestUsuarioComPerfilEVendedorVinculado(t *testing.T) {
	a, n := logadoAdmin(t)
	a.banco.W.Exec(`INSERT INTO Pessoas (ups_nome, ups_vendedor) VALUES ('Carlos', 1), ('Cliente', 0)`)
	n.postar("/usuarios/salvar", usuarioForm("Carlos", "carlos", "vendedorId", "1", "perfil", "VENDEDOR_LOJA"))
	d := a.direitosDe("carlos")
	if d["PESSOAS"] != "CIA" || d["DOCUMENTOS"] != "CIA" || d["APP_VENDAS"] != "CI" || d["BI_EXECUTIVO"] != "" || d["VENDAS_TODOS"] != "" {
		t.Errorf("perfil Vendedor da loja não aplicado: %v", d)
	}
	var vend int64
	a.banco.R.QueryRow(`SELECT usu_ups_vendedor FROM Usuarios WHERE usu_login = 'carlos'`).Scan(&vend)
	if vend != 1 {
		t.Errorf("vendedor vinculado = %d", vend)
	}
	// a lista mostra o nome do vendedor; a pessoa que não é vendedora é recusada
	if !strings.Contains(n.paginaOK("/usuarios"), "<td>Carlos</td>") {
		t.Error("a lista deveria mostrar o vendedor vinculado")
	}
	n.postar("/usuarios/salvar", usuarioForm("X", "x", "vendedorId", "2"))
	n.esperaMensagem("/usuarios", "Erro: A pessoa &#34;Cliente&#34; não está cadastrada como vendedor ativo.")
	n.postar("/usuarios/salvar", usuarioForm("X", "x", "vendedorId", "77"))
	n.esperaMensagem("/usuarios", "Erro: Vendedor não encontrado.")
}

func TestUsuarioValidacoes(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Ana", "ana"))
	n.postar("/usuarios/salvar", usuarioForm("Outra", "  ANA  "))
	n.esperaMensagem("/usuarios", "Erro: Já existe um usuário com o login &#34;ANA&#34;.")
	n.postar("/usuarios/salvar", usuarioForm("Outra", "AnA"+" "))
	// senha em branco para usuário novo: volta ao formulário só com o aviso da senha
	v := usuarioForm("Sem senha", "semsenha", "novaSenha", "  ")
	v.Set("_csrf", n.csrfLogado())
	resp, corpo := n.fazer("POST", "/usuarios/salvar", v, nil)
	if resp.StatusCode != 200 || !strings.Contains(corpo, `<span class="field-error">Senha é obrigatória para novos usuários.</span>`) || !strings.Contains(corpo, `value="semsenha"`) {
		t.Errorf("senha em branco: status %d", resp.StatusCode)
	}
	// nome ou login inválido: volta ao formulário sem mensagem
	for _, v := range []url.Values{usuarioForm(" ", "x"), usuarioForm("N", ""), usuarioForm(strings.Repeat("N", 151), "x"), usuarioForm("N", strings.Repeat("l", 81))} {
		v.Set("_csrf", n.csrfLogado())
		resp, corpo := n.fazer("POST", "/usuarios/salvar", v, nil)
		if resp.StatusCode != 200 || strings.Contains(corpo, "field-error") || !strings.Contains(corpo, "<span>Usuário</span>") {
			t.Errorf("validação: status %d", resp.StatusCode)
		}
	}
	if a.conta(`SELECT count(*) FROM Usuarios`) != 2 { // admin + ana
		t.Error("nenhum usuário inválido deveria ter sido gravado")
	}
}

func TestUsuarioEditarMantemSenhaEPermissoes(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Ana", "ana", "perfil", "RECEPCAO"))
	antes := a.direitosDe("ana")
	var h1 string
	a.banco.R.QueryRow(`SELECT usu_senha FROM Usuarios WHERE usu_login = 'ana'`).Scan(&h1)
	n.postar("/usuarios/salvar", usuarioForm("Ana Maria", "ana.maria", "usuCodigo", "2", "novaSenha", ""))
	var h2 string
	a.banco.R.QueryRow(`SELECT usu_senha FROM Usuarios WHERE usu_codigo = 2`).Scan(&h2)
	if h1 != h2 {
		t.Error("senha em branco na edição mantém a senha")
	}
	depois := a.direitosDe("ana.maria")
	if len(depois) != len(antes) {
		t.Errorf("editar não pode mexer nas permissões: antes %v, depois %v", antes, depois)
	}
	n.postar("/usuarios/salvar", usuarioForm("Ana Maria", "ana.maria", "usuCodigo", "2", "novaSenha", "Nova-Senha-456"))
	m := a.novoNavegador()
	if resp := m.login("ana.maria", "Senha-Boa-123"); resp.Header.Get("Location") != "/login?error=true" {
		t.Error("a senha antiga não deveria mais valer")
	}
	m2 := a.novoNavegador()
	m2.entrar("ana.maria", "Nova-Senha-456")
	// o perfil escolhido na edição é ignorado (só vale na criação)
	n.postar("/usuarios/salvar", usuarioForm("Ana Maria", "ana.maria", "usuCodigo", "2", "novaSenha", "", "perfil", "GERENCIA"))
	if a.direitosDe("ana.maria")["BI_EXECUTIVO"] != "" {
		t.Error("o perfil não se aplica na edição")
	}
	n.postar("/usuarios/salvar", usuarioForm("Fantasma", "fantasma", "usuCodigo", "99", "novaSenha", ""))
	n.esperaMensagem("/usuarios", "Erro: Usuário não encontrado.")
	n.postar("/usuarios/salvar", usuarioForm("Ana Maria", "admin", "usuCodigo", "2", "novaSenha", ""))
	n.esperaMensagem("/usuarios", "Erro: Já existe um usuário com o login &#34;admin&#34;.")
}

func TestAdminNaoMudaLoginNemInativaNemExclui(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Administrador", "chefe", "usuCodigo", "1", "novaSenha", ""))
	n.esperaMensagem("/usuarios", "Erro: O login do administrador não pode ser alterado.")
	n.postar("/usuarios/salvar", usuarioForm("Administrador", "ADMIN", "usuCodigo", "1", "novaSenha", ""))
	n.esperaMensagem("/usuarios", "Usuário salvo com sucesso!") // só trocar a caixa é permitido
	v := usuarioForm("Administrador", "admin", "usuCodigo", "1", "novaSenha", "")
	v.Del("usuAtivo")
	n.postar("/usuarios/salvar", v)
	n.esperaMensagem("/usuarios", "Erro: O administrador não pode ser inativado.")
	n.postar("/usuarios/status/1", url.Values{"ativo": {"false"}})
	n.esperaMensagem("/usuarios", "O administrador não pode ser inativado.")
	n.postar("/usuarios/excluir/1", nil)
	n.esperaMensagem("/usuarios", "O administrador não pode ser excluído.")
	if a.conta(`SELECT count(*) FROM Usuarios WHERE usu_codigo = 1 AND usu_ativo = 1`) != 1 {
		t.Error("o admin continua ativo")
	}
}

func TestUsuarioInativoNaoEntraEStatusSeguePelaLista(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Ana", "ana"))
	n.postar("/usuarios/status/2", url.Values{"ativo": {"false"}})
	n.esperaMensagem("/usuarios", "Status do usuário atualizado!")
	if !strings.Contains(n.paginaOK("/usuarios"), "Inativo") {
		t.Error("deveria aparecer inativo")
	}
	if resp := a.novoNavegador().login("ana", "Senha-Boa-123"); resp.Header.Get("Location") != "/login?error=true" {
		t.Error("usuário inativo não entra")
	}
	n.postar("/usuarios/status/2", url.Values{"ativo": {"true"}})
	a.novoNavegador().entrar("ana", "Senha-Boa-123")
	n.postar("/usuarios/status/999", url.Values{"ativo": {"true"}})
	n.esperaMensagem("/usuarios", "Status do usuário atualizado!")
}

func TestUsuarioExcluirComDocumentoBarra(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Lança", "lanca"))
	n.postar("/usuarios/salvar", usuarioForm("Livre", "livre"))
	a.banco.W.Exec(`INSERT INTO Pessoas (ups_nome, ups_fornecedor) VALUES ('Forn', 1)`)
	a.banco.W.Exec(`INSERT INTO Documento (docto_tipo, docto_data, docto_valor, docto_ups_codigo, docto_usu_codigo) VALUES ('COMPRA','2026-11-20',100,1,2)`)
	n.postar("/usuarios/excluir/2", nil)
	n.esperaMensagem("/usuarios", "Não foi possível excluir.")
	if a.conta(`SELECT count(*) FROM Usuarios WHERE usu_codigo = 2`) != 1 || len(a.direitosDe("lanca")["#linhas"]) != 21 {
		t.Error("o usuário com documento (e os direitos dele) deve permanecer inteiro")
	}
	n.postar("/usuarios/excluir/3", nil)
	n.esperaMensagem("/usuarios", "Usuário excluído!")
	if a.conta(`SELECT count(*) FROM Direito_Usuario WHERE dusu_usu_codigo = 3`) != 0 {
		t.Error("os direitos do usuário excluído deveriam sumir junto")
	}
	n.postar("/usuarios/excluir/999", nil)
	n.esperaMensagem("/usuarios", "Usuário excluído!")
}

func TestUsuarioNovoComPerfilInexistenteDesfazEQueimaOCodigo(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Falha", "falha", "perfil", "NADA"))
	n.esperaMensagem("/usuarios", "Erro: Perfil não encontrado.")
	if a.conta(`SELECT count(*) FROM Usuarios WHERE usu_login = 'falha'`) != 0 || a.conta(`SELECT count(*) FROM Direito_Usuario WHERE dusu_usu_codigo = 2`) != 0 {
		t.Error("o cadastro deveria ter sido desfeito por inteiro")
	}
	n.postar("/usuarios/salvar", usuarioForm("Depois", "depois"))
	if a.conta(`SELECT count(*) FROM Usuarios WHERE usu_login = 'depois' AND usu_codigo = 3`) != 1 {
		t.Error("o código 2 fica queimado: o próximo usuário é o 3 (como o IDENTITY do antigo)")
	}
}

func TestPermissoesTelaEMatriz(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Ana", "ana"))
	corpo := n.paginaOK("/usuarios/permissoes/2")
	for _, s := range []string{"Permissões — Ana", "Configurando permissões de:", `name="consulta_PRODUTOS" checked="checked"`, `name="inclusao_PRODUTOS"`, "Aplicar um perfil pronto", "Vendedor da loja"} {
		if !strings.Contains(corpo, s) {
			t.Errorf("tela de permissões sem %q", s)
		}
	}
	// módulos só de Consulta não oferecem as outras colunas; Metas não oferece Exclusão nem Atualização
	for _, nao := range []string{`name="inclusao_VENDAS_TODOS"`, `name="atualizacao_COMISSOES"`, `name="exclusao_AGENDA_TODOS"`, `name="exclusao_METAS"`, `name="atualizacao_METAS"`} {
		if strings.Contains(corpo, nao) {
			t.Errorf("não deveria oferecer %s", nao)
		}
	}
	if !strings.Contains(corpo, `name="inclusao_METAS"`) {
		t.Error("Metas oferece Inclusão")
	}
	// admin: tudo marcado e desabilitado, sem perfis e com o botão desabilitado
	adm := n.paginaOK("/usuarios/permissoes/1")
	if !strings.Contains(adm, "sempre tem acesso total") || strings.Contains(adm, "Aplicar um perfil pronto") || !strings.Contains(adm, `disabled="disabled"><i class="fas fa-save">`) {
		t.Error("tela do admin incorreta")
	}
	// salvar a matriz: presente = marcado, ausente = desmarcado
	n.postar("/usuarios/permissoes/2", url.Values{"consulta_PRODUTOS": {"on"}, "inclusao_PRODUTOS": {"on"}, "atualizacao_PRODUTOS": {"on"}, "consulta_FIN_DRE": {"on"}})
	n.esperaMensagem("/usuarios/permissoes/2", "Permissões salvas! Valem na hora para o usuário.")
	d := a.direitosDe("ana")
	if d["PRODUTOS"] != "CIA" || d["FIN_DRE"] != "C" || d["PESSOAS"] != "" || d["ESTOQUE"] != "" {
		t.Errorf("matriz gravada incorreta: %v", d)
	}
}

func TestPermissoesValemNaHoraParaQuemJaEstaLogado(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Ana", "ana"))
	ana := a.novoNavegador()
	ana.entrar("ana", "Senha-Boa-123")
	if resp, _ := ana.get("/produtos"); resp.StatusCode != 200 {
		t.Fatal("Ana consulta produtos")
	}
	n.postar("/usuarios/permissoes/2", url.Values{"consulta_PESSOAS": {"on"}}) // tira Produtos
	if resp, _ := ana.get("/produtos"); resp.StatusCode != 302 || resp.Header.Get("Location") != "/dashboard" {
		t.Errorf("sem login novo, a perda de permissão já deveria valer: %d", resp.StatusCode)
	}
	ana.esperaMensagem("/dashboard", "Você não tem permissão para acessar Produtos.")
}

func TestAplicarPerfilSubstituiTudoENaoVaiParaOAdmin(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Ana", "ana"))
	n.postar("/usuarios/permissoes/2", url.Values{"consulta_BI_EXECUTIVO": {"on"}, "exclusao_PRODUTOS": {"on"}})
	n.postar("/usuarios/permissoes/2/perfil", url.Values{"perfil": {"PROFISSIONAL_SALAO"}})
	n.esperaMensagem("/usuarios/permissoes/2", "Perfil aplicado! Valem na hora para o usuário. Ajuste abaixo o que precisar.")
	d := a.direitosDe("ana")
	if d["BI_EXECUTIVO"] != "" || d["PRODUTOS"] == "CE" || d["AGENDA"] == "" || d["COMISSOES"] != "C" {
		t.Errorf("o perfil deveria substituir tudo: %v", d)
	}
	n.postar("/usuarios/permissoes/2/perfil", url.Values{"perfil": {"NADA"}})
	n.esperaMensagem("/usuarios/permissoes/2", "Não foi possível aplicar o perfil: Perfil não encontrado.")
	n.postar("/usuarios/permissoes/1/perfil", url.Values{"perfil": {"RECEPCAO"}})
	n.esperaMensagem("/usuarios/permissoes/1", "Não foi possível aplicar o perfil: O administrador sempre tem acesso total; não há perfil a aplicar.")
	if st, _ := n.postar("/usuarios/permissoes/2/perfil", nil); st != 400 {
		t.Errorf("sem o parâmetro perfil = %d, esperado 400", st)
	}
	n.get("/usuarios/permissoes/999")
	n.esperaMensagem("/usuarios", "Usuário não encontrado.")
}

func TestModuloSemLinhaApareceZeradoNasPermissoes(t *testing.T) {
	a, n := logadoAdmin(t)
	n.postar("/usuarios/salvar", usuarioForm("Ana", "ana"))
	a.banco.W.Exec(`DELETE FROM Direito_Usuario WHERE dusu_usu_codigo = 2 AND dusu_descricao IN ('BI_INSIGHTS','METAS')`)
	n.paginaOK("/usuarios/permissoes/2")
	if a.conta(`SELECT count(*) FROM Direito_Usuario WHERE dusu_usu_codigo = 2`) != 21 {
		t.Error("módulos criados depois do cadastro do usuário entram zerados na tela de permissões")
	}
	if a.direitosDe("ana")["METAS"] != "" {
		t.Error("e sem nenhum acesso")
	}
}

func TestUsuariosEPermissoesRespeitamAsPermissoes(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.criarUsuario("leitor", "Leitor", "Leitor-Senha-1", true, "USUARIOS:C")
	a.criarUsuario("gestor", "Gestor", "Gestor-Senha-1", true, "USUARIOS:CIAE", "PERMISSOES:CA")
	l := a.novoNavegador()
	l.entrar("leitor", "Leitor-Senha-1")
	corpo := l.paginaOK("/usuarios")
	for _, nao := range []string{"Novo Usuário", "/usuarios/editar/", "/usuarios/excluir/", "/usuarios/status/", "/usuarios/permissoes/"} {
		if strings.Contains(corpo, nao) {
			t.Errorf("quem só consulta usuários não deveria ver %q", nao)
		}
	}
	if resp, _ := l.get("/usuarios/permissoes/1"); resp.Header.Get("Location") != "/dashboard" {
		t.Error("a tela de permissões exige Consulta em Permissões")
	}
	l.esperaMensagem("/dashboard", "Você não tem permissão para acessar Permissões.")
	g := a.novoNavegador()
	g.entrar("gestor", "Gestor-Senha-1")
	if !strings.Contains(g.paginaOK("/usuarios"), "/usuarios/permissoes/") {
		t.Error("quem consulta Permissões vê o atalho")
	}
	g.paginaOK("/usuarios/permissoes/1")
	if st, loc := g.postar("/usuarios/permissoes/2", url.Values{"consulta_PRODUTOS": {"on"}}); st != 302 || loc != "/usuarios/permissoes/2" {
		t.Errorf("salvar matriz com Atualização = %d → %s", st, loc)
	}
	l.postar("/usuarios/permissoes/2", url.Values{"consulta_PRODUTOS": {"on"}})
	l.esperaMensagem("/dashboard", "Você não tem permissão para alterar registros em Permissões.")
}
