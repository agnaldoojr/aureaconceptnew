package permissoes

var icones = map[string]string{
	"PRODUTOS": "fa-gem", "UNIDADES": "fa-ruler", "PESSOAS": "fa-users", "DOCUMENTOS": "fa-file-invoice", "VENDAS_TODOS": "fa-eye",
	"COMISSOES": "fa-percent", "AGENDA": "fa-calendar-days", "AGENDA_TODOS": "fa-eye", "ESTOQUE": "fa-boxes-stacked",
	"RELATORIOS": "fa-chart-bar", "BI_EXECUTIVO": "fa-chart-pie", "BI_INSIGHTS": "fa-lightbulb", "METAS": "fa-bullseye",
	"FIN_TITULOS": "fa-file-invoice-dollar", "FIN_CONTAS": "fa-building-columns", "FIN_FLUXO": "fa-money-bill-trend-up",
	"FIN_DRE": "fa-scale-balanced", "FIN_CADASTROS": "fa-list-check", "APP_VENDAS": "fa-mobile-screen-button",
	"USUARIOS": "fa-user-shield", "PERMISSOES": "fa-key",
}

// Icone é a classe Font Awesome do módulo na tela de permissões.
func Icone(modulo string) string { return icones[modulo] }

// EhAdmin diz se o login é o do administrador (comparado sem diferenciar maiúscula).
func EhAdmin(login string) bool { return equalFold(login, LoginAdmin) }
