package http

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"pre-ipo-market-platform/internal/domain"
	"pre-ipo-market-platform/internal/i18n"
	"pre-ipo-market-platform/internal/store"
)

//go:embed static/* templates/*
var content embed.FS

type Server struct {
	store     *store.Store
	templates *template.Template
}

type pageData struct {
	Title         string
	User          domain.User
	Lang          string
	Flash         string
	Error         string
	Companies     []domain.Company
	Company       domain.Company
	SellOrders    []domain.SellOrder
	BuyInterests  []domain.BuyInterest
	Transactions  []domain.Transaction
	Negotiations  []domain.Negotiation
	Deals         []domain.Deal
	Subscriptions []domain.Subscription
	Holdings      []domain.Holding
	Users         []domain.User
	SPVs          []domain.SPVVehicle
	Documents     []domain.ExecutionDocument
	Valuations    []domain.ValuationRecord
	ExitEvents    []domain.ExitEvent
	Distributions []domain.Distribution
	Reports       []domain.InvestorReport
	RiskAlerts    []domain.RiskAlert
	Tickets       []domain.SupportTicket
	PendingUsers  []domain.User
	AuditLogs     []domain.AuditLog
	Stats         map[string]int
}

func NewServer(store *store.Store) *Server {
	funcs := template.FuncMap{
		"t": i18n.T,
		"money": func(v float64) string {
			return fmt.Sprintf("$%.0f", v)
		},
		"canAdmin":    domain.CanAccessAdmin,
		"canBuy":      domain.CanSubmitBuyInterest,
		"canSell":     domain.CanSubmitSellOrder,
		"switchLang":  switchLang,
		"statusLabel": statusLabel,
		"percent":     percent,
		"terminalTxn": func(s domain.TransactionStage) bool { return s == domain.StageSettled || s == domain.StageCancelled },
		"terminalSub": func(s domain.SubscriptionStatus) bool {
			return s == domain.SubscriptionActive || s == domain.SubscriptionCancelled
		},
	}
	tpl := template.Must(template.New("").Funcs(funcs).ParseFS(content, "templates/*.html"))
	return &Server{store: store, templates: tpl}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(content)))
	mux.HandleFunc("/", s.requireAuth(s.dashboard))
	mux.HandleFunc("/login", s.loginPage)
	mux.HandleFunc("/auth/login", s.login)
	mux.HandleFunc("/logout", s.logout)
	mux.HandleFunc("/language", s.requireAuth(s.language))
	mux.HandleFunc("/dashboard", s.requireAuth(s.dashboard))
	mux.HandleFunc("/companies", s.requireAuth(s.companies))
	mux.HandleFunc("/companies/", s.requireAuth(s.companyDetail))
	mux.HandleFunc("/market/orders", s.requireAuth(s.market))
	mux.HandleFunc("/orders/sell", s.requireAuth(s.createSellOrder))
	mux.HandleFunc("/orders/buy-interest", s.requireAuth(s.createBuyInterest))
	mux.HandleFunc("/negotiations/create", s.requireAuth(s.createNegotiation))
	mux.HandleFunc("/deals", s.requireAuth(s.deals))
	mux.HandleFunc("/deals/", s.requireAuth(s.dealActions))
	mux.HandleFunc("/portfolio", s.requireAuth(s.portfolio))
	mux.HandleFunc("/support/tickets", s.requireAuth(s.createSupportTicket))
	mux.HandleFunc("/admin", s.requireAdmin(s.admin))
	mux.HandleFunc("/admin/companies/create", s.requireAdmin(s.createCompany))
	mux.HandleFunc("/admin/deals/create", s.requireAdmin(s.createDeal))
	mux.HandleFunc("/admin/matches/create", s.requireAdmin(s.createMatch))
	mux.HandleFunc("/admin/documents/create", s.requireAdmin(s.createDocument))
	mux.HandleFunc("/admin/documents/", s.requireAdmin(s.advanceDocument))
	mux.HandleFunc("/admin/valuations/create", s.requireAdmin(s.createValuation))
	mux.HandleFunc("/admin/exits/create", s.requireAdmin(s.createExitEvent))
	mux.HandleFunc("/admin/distributions/create", s.requireAdmin(s.createDistribution))
	mux.HandleFunc("/admin/reports/create", s.requireAdmin(s.createReport))
	mux.HandleFunc("/admin/risks/create", s.requireAdmin(s.createRiskAlert))
	mux.HandleFunc("/admin/risks/", s.requireAdmin(s.resolveRiskAlert))
	mux.HandleFunc("/admin/tickets/", s.requireAdmin(s.closeTicket))
	mux.HandleFunc("/admin/reviews/", s.requireAdmin(s.approveReview))
	mux.HandleFunc("/admin/transactions/", s.requireAdmin(s.advanceTransaction))
	mux.HandleFunc("/admin/subscriptions/", s.requireAdmin(s.advanceSubscription))
	return mux
}

type handlerFunc func(http.ResponseWriter, *http.Request, domain.User)

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data pageData) {
	if data.Lang == "" {
		data.Lang = "zh"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) requireAuth(next handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.currentUser(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r, user)
	}
}

func (s *Server) requireAdmin(next handlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request, user domain.User) {
		if !domain.CanAccessAdmin(user.Role) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r, user)
	})
}

func (s *Server) currentUser(r *http.Request) (domain.User, bool) {
	cookie, err := r.Cookie("preipo_session")
	if err != nil || cookie.Value == "" {
		return domain.User{}, false
	}
	user, err := s.store.UserBySession(cookie.Value)
	return user, err == nil
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.render(w, r, "login.html", pageData{Title: "Login", Lang: "zh", Error: r.URL.Query().Get("error")})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=form", http.StatusSeeOther)
		return
	}
	user, err := s.store.Authenticate(r.FormValue("email"), r.FormValue("password"))
	if err != nil {
		http.Redirect(w, r, "/login?error=invalid", http.StatusSeeOther)
		return
	}
	token := uuid.NewString()
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := s.store.CreateSession(user.ID, token, expiresAt); err != nil {
		http.Redirect(w, r, "/login?error=session", http.StatusSeeOther)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "preipo_session", Value: token, Path: "/", Expires: expiresAt, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("preipo_session"); err == nil {
		_ = s.store.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "preipo_session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) language(w http.ResponseWriter, r *http.Request, user domain.User) {
	lang := r.URL.Query().Get("lang")
	_ = s.store.SetLanguage(user.ID, lang)
	redirect := r.Header.Get("Referer")
	if redirect == "" {
		redirect = "/dashboard"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request, user domain.User) {
	companies, _ := s.store.Companies()
	transactions, _ := s.store.Transactions(user)
	deals, _ := s.store.Deals()
	holdings, _ := s.store.Holdings(user.ID)
	stats := map[string]int{
		"companies":    len(companies),
		"transactions": len(transactions),
		"deals":        len(deals),
		"holdings":     len(holdings),
	}
	s.render(w, r, "dashboard.html", pageData{Title: "Dashboard", User: user, Lang: user.Language, Companies: companies, Transactions: transactions, Deals: deals, Holdings: holdings, Stats: stats})
}

func (s *Server) companies(w http.ResponseWriter, r *http.Request, user domain.User) {
	companies, err := s.store.Companies()
	data := pageData{Title: "Companies", User: user, Lang: user.Language, Companies: companies}
	if err != nil {
		data.Error = err.Error()
	}
	s.render(w, r, "companies.html", data)
}

func (s *Server) createCompany(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	sharePrice, _ := strconv.ParseFloat(r.FormValue("share_price"), 64)
	company := domain.Company{
		Name:                 r.FormValue("name"),
		Industry:             r.FormValue("industry"),
		Valuation:            r.FormValue("valuation"),
		FundingRound:         r.FormValue("funding_round"),
		SharePrice:           sharePrice,
		Description:          r.FormValue("description"),
		TradableStatus:       r.FormValue("tradable_status"),
		TransferRestrictions: r.FormValue("transfer_restrictions"),
	}
	if company.Name == "" || company.Industry == "" || company.Valuation == "" {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateCompany(r.Context(), user.ID, company); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) companyDetail(w http.ResponseWriter, r *http.Request, user domain.User) {
	id, err := pathID(r.URL.Path, "/companies/")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	company, err := s.store.Company(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "company.html", pageData{Title: company.Name, User: user, Lang: user.Language, Company: company})
}

func (s *Server) market(w http.ResponseWriter, r *http.Request, user domain.User) {
	companies, _ := s.store.Companies()
	sellOrders, _ := s.store.SellOrders(user)
	buyInterests, _ := s.store.BuyInterests(user)
	transactions, _ := s.store.Transactions(user)
	negotiations, _ := s.store.Negotiations(user)
	s.render(w, r, "market.html", pageData{Title: "Market", User: user, Lang: user.Language, Companies: companies, SellOrders: sellOrders, BuyInterests: buyInterests, Transactions: transactions, Negotiations: negotiations, Error: r.URL.Query().Get("error")})
}

func (s *Server) createSellOrder(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !domain.CanSubmitSellOrder(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	companyID, shares, price, err := parseOrderForm(r)
	if err != nil {
		http.Redirect(w, r, "/market/orders?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateSellOrder(r.Context(), user.ID, companyID, shares, price); err != nil {
		http.Redirect(w, r, "/market/orders?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/market/orders", http.StatusSeeOther)
}

func (s *Server) createBuyInterest(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !domain.CanSubmitBuyInterest(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/market/orders?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	price, _ := strconv.ParseFloat(r.FormValue("target_price"), 64)
	if companyID <= 0 || amount <= 0 || price <= 0 {
		http.Redirect(w, r, "/market/orders?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateBuyInterest(r.Context(), user.ID, companyID, amount, price); err != nil {
		http.Redirect(w, r, "/market/orders?error=create", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/market/orders", http.StatusSeeOther)
}

func (s *Server) createNegotiation(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/market/orders?error=form", http.StatusSeeOther)
		return
	}
	transactionID, _ := strconv.ParseInt(r.FormValue("transaction_id"), 10, 64)
	offerPrice, _ := strconv.ParseFloat(r.FormValue("offer_price"), 64)
	shares, _ := strconv.ParseInt(r.FormValue("shares"), 10, 64)
	note := r.FormValue("note")
	redirect := r.FormValue("redirect")
	if redirect == "" {
		redirect = "/market/orders"
	}
	if err := s.store.CreateNegotiation(r.Context(), user, transactionID, offerPrice, shares, note); err != nil {
		http.Redirect(w, r, redirect+"?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *Server) deals(w http.ResponseWriter, r *http.Request, user domain.User) {
	deals, _ := s.store.Deals()
	spvs, _ := s.store.SPVVehicles()
	subscriptions, _ := s.store.Subscriptions(user)
	s.render(w, r, "deals.html", pageData{Title: "Deals", User: user, Lang: user.Language, Deals: deals, SPVs: spvs, Subscriptions: subscriptions, Error: r.URL.Query().Get("error")})
}

func (s *Server) createDeal(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	minimum, _ := strconv.ParseFloat(r.FormValue("min_subscription"), 64)
	target, _ := strconv.ParseFloat(r.FormValue("target_size"), 64)
	deal := domain.Deal{
		CompanyID:       companyID,
		Name:            r.FormValue("name"),
		DealType:        r.FormValue("deal_type"),
		Structure:       r.FormValue("structure"),
		MinSubscription: minimum,
		TargetSize:      target,
		FeeDescription:  r.FormValue("fee_description"),
	}
	if deal.CompanyID <= 0 || deal.Name == "" || deal.DealType == "" || deal.MinSubscription <= 0 || deal.TargetSize <= 0 {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateDeal(r.Context(), user.ID, deal); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) dealActions(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/subscribe") {
		http.NotFound(w, r)
		return
	}
	if !domain.CanSubmitBuyInterest(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/deals/"), "/subscribe")
	dealID, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/deals?error=form", http.StatusSeeOther)
		return
	}
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err := s.store.CreateSubscription(r.Context(), user.ID, dealID, amount); err != nil {
		http.Redirect(w, r, "/deals?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/deals", http.StatusSeeOther)
}

func (s *Server) portfolio(w http.ResponseWriter, r *http.Request, user domain.User) {
	holdings, _ := s.store.Holdings(user.ID)
	transactions, _ := s.store.Transactions(user)
	negotiations, _ := s.store.Negotiations(user)
	documents, _ := s.store.ExecutionDocuments(user)
	subscriptions, _ := s.store.Subscriptions(user)
	valuations, _ := s.store.Valuations()
	exitEvents, _ := s.store.ExitEvents()
	distributions, _ := s.store.Distributions(user.ID)
	reports, _ := s.store.Reports(user.ID)
	tickets, _ := s.store.SupportTickets(user.ID, false)
	s.render(w, r, "portfolio.html", pageData{Title: "Portfolio", User: user, Lang: user.Language, Holdings: holdings, Transactions: transactions, Negotiations: negotiations, Documents: documents, Subscriptions: subscriptions, Valuations: valuations, ExitEvents: exitEvents, Distributions: distributions, Reports: reports, Tickets: tickets})
}

func (s *Server) createSupportTicket(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/portfolio?error=form", http.StatusSeeOther)
		return
	}
	subject := r.FormValue("subject")
	note := r.FormValue("note")
	if subject == "" {
		http.Redirect(w, r, "/portfolio?error=form", http.StatusSeeOther)
		return
	}
	_ = s.store.CreateSupportTicket(r.Context(), user.ID, subject, note)
	http.Redirect(w, r, "/portfolio", http.StatusSeeOther)
}

func (s *Server) admin(w http.ResponseWriter, r *http.Request, user domain.User) {
	pending, _ := s.store.UsersPendingReview()
	users, _ := s.store.Users()
	companies, _ := s.store.Companies()
	sellOrders, _ := s.store.SellOrders(user)
	buyInterests, _ := s.store.BuyInterests(user)
	transactions, _ := s.store.Transactions(user)
	negotiations, _ := s.store.Negotiations(user)
	deals, _ := s.store.Deals()
	spvs, _ := s.store.SPVVehicles()
	subscriptions, _ := s.store.Subscriptions(user)
	documents, _ := s.store.ExecutionDocuments(user)
	valuations, _ := s.store.Valuations()
	exitEvents, _ := s.store.ExitEvents()
	riskAlerts, _ := s.store.RiskAlerts()
	tickets, _ := s.store.SupportTickets(user.ID, true)
	logs, _ := s.store.AuditLogs(20)
	s.render(w, r, "admin.html", pageData{Title: "Admin", User: user, Lang: user.Language, Users: users, Companies: companies, PendingUsers: pending, SellOrders: sellOrders, BuyInterests: buyInterests, Transactions: transactions, Negotiations: negotiations, Deals: deals, SPVs: spvs, Subscriptions: subscriptions, Documents: documents, Valuations: valuations, ExitEvents: exitEvents, RiskAlerts: riskAlerts, Tickets: tickets, AuditLogs: logs, Error: r.URL.Query().Get("error")})
}

func (s *Server) createMatch(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	sellOrderID, _ := strconv.ParseInt(r.FormValue("sell_order_id"), 10, 64)
	buyInterestID, _ := strconv.ParseInt(r.FormValue("buy_interest_id"), 10, 64)
	shares, _ := strconv.ParseInt(r.FormValue("shares"), 10, 64)
	price, _ := strconv.ParseFloat(r.FormValue("price"), 64)
	if err := s.store.CreateMatchedTransaction(r.Context(), user.ID, sellOrderID, buyInterestID, shares, price); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createDocument(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	transactionID, _ := strconv.ParseInt(r.FormValue("transaction_id"), 10, 64)
	documentType := r.FormValue("document_type")
	note := r.FormValue("note")
	if err := s.store.CreateExecutionDocument(r.Context(), user.ID, transactionID, documentType, note); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) advanceDocument(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/advance") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/documents/"), "/advance")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.AdvanceExecutionDocument(r.Context(), user.ID, id)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createValuation(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	sharePrice, _ := strconv.ParseFloat(r.FormValue("share_price"), 64)
	if companyID <= 0 || r.FormValue("valuation") == "" || sharePrice <= 0 {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateValuation(r.Context(), user.ID, companyID, r.FormValue("valuation"), sharePrice, r.FormValue("as_of_date")); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createExitEvent(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	event := domain.ExitEvent{
		CompanyID:    companyID,
		EventType:    r.FormValue("event_type"),
		Description:  r.FormValue("description"),
		Status:       r.FormValue("status"),
		ExpectedDate: r.FormValue("expected_date"),
	}
	if event.CompanyID <= 0 || event.EventType == "" || event.Status == "" {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateExitEvent(r.Context(), user.ID, event); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createDistribution(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	userID, _ := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	distribution := domain.Distribution{UserID: userID, Amount: amount, Status: r.FormValue("status"), TaxDocument: r.FormValue("tax_document")}
	if distribution.UserID <= 0 || distribution.Amount < 0 || distribution.Status == "" {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateDistribution(r.Context(), user.ID, distribution); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createReport(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	userID, _ := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	report := domain.InvestorReport{UserID: userID, ReportType: r.FormValue("report_type"), Title: r.FormValue("title"), Period: r.FormValue("period"), Status: r.FormValue("status")}
	if report.UserID <= 0 || report.Title == "" || report.ReportType == "" {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateReport(r.Context(), user.ID, report); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createRiskAlert(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	alert := domain.RiskAlert{Severity: r.FormValue("severity"), Status: "open", Subject: r.FormValue("subject"), Note: r.FormValue("note")}
	if alert.Severity == "" || alert.Subject == "" {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateRiskAlert(r.Context(), user.ID, alert); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) resolveRiskAlert(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/resolve") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/risks/"), "/resolve")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.ResolveRiskAlert(r.Context(), user.ID, id)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) closeTicket(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/close") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/tickets/"), "/close")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.CloseSupportTicket(r.Context(), user.ID, id)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) approveReview(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	action := ""
	switch {
	case strings.HasSuffix(r.URL.Path, "/approve"):
		action = "approve"
	case strings.HasSuffix(r.URL.Path, "/reject"):
		action = "reject"
	default:
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/reviews/"), "/approve"), "/reject")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if action == "approve" {
		_ = s.store.ApproveUser(r.Context(), user.ID, id)
	} else {
		_ = s.store.RejectUser(r.Context(), user.ID, id)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) advanceTransaction(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	action := ""
	switch {
	case strings.HasSuffix(r.URL.Path, "/advance"):
		action = "advance"
	case strings.HasSuffix(r.URL.Path, "/cancel"):
		action = "cancel"
	default:
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/transactions/"), "/advance"), "/cancel")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if action == "advance" {
		_ = s.store.AdvanceTransaction(r.Context(), user.ID, id)
	} else {
		_ = s.store.CancelTransaction(r.Context(), user.ID, id)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) advanceSubscription(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	action := ""
	switch {
	case strings.HasSuffix(r.URL.Path, "/advance"):
		action = "advance"
	case strings.HasSuffix(r.URL.Path, "/cancel"):
		action = "cancel"
	default:
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/subscriptions/"), "/advance"), "/cancel")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if action == "advance" {
		_ = s.store.AdvanceSubscription(r.Context(), user.ID, id)
	} else {
		_ = s.store.CancelSubscription(r.Context(), user.ID, id)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func parseOrderForm(r *http.Request) (int64, int64, float64, error) {
	if err := r.ParseForm(); err != nil {
		return 0, 0, 0, err
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	shares, _ := strconv.ParseInt(r.FormValue("shares"), 10, 64)
	price, _ := strconv.ParseFloat(r.FormValue("target_price"), 64)
	if companyID <= 0 || shares <= 0 || price <= 0 {
		return 0, 0, 0, fmt.Errorf("invalid order")
	}
	return companyID, shares, price, nil
}

func pathID(path, prefix string) (int64, error) {
	idText := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	return strconv.ParseInt(idText, 10, 64)
}

func switchLang(lang string) string {
	if lang == "en" {
		return "zh"
	}
	return "en"
}

func statusLabel(status any) string {
	return strings.ReplaceAll(fmt.Sprint(status), "_", " ")
}

func percent(value, total any) string {
	valueFloat := toFloat(value)
	totalFloat := toFloat(total)
	if totalFloat <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", valueFloat/totalFloat*100)
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	default:
		return 0
	}
}

func urlSafe(v string) string {
	return strings.ReplaceAll(strings.ReplaceAll(v, " ", "+"), "/", "-")
}
