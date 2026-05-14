package http

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"pre-ipo-market-platform/internal/buildinfo"
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
	Title             string
	User              domain.User
	Lang              string
	Flash             string
	Error             string
	Companies         []domain.Company
	Company           domain.Company
	SelectedCompany   domain.Company
	SelectedCompanyID int64
	Watchlist         []domain.WatchlistItem
	WatchlistMap      map[int64]bool
	ComplianceReviews []domain.ComplianceReview
	SellOrders        []domain.SellOrder
	BuyInterests      []domain.BuyInterest
	Transactions      []domain.Transaction
	OpenTransactions  []domain.Transaction
	Negotiations      []domain.Negotiation
	Deals             []domain.Deal
	Subscriptions     []domain.Subscription
	SubDocuments      []domain.SubscriptionDocument
	Holdings          []domain.Holding
	PortfolioValues   []domain.PortfolioValuation
	PortfolioSummary  domain.PortfolioSummary
	Users             []domain.User
	SPVs              []domain.SPVVehicle
	Documents         []domain.ExecutionDocument
	Approvals         []domain.ExecutionApproval
	EscrowPayments    []domain.EscrowPayment
	Valuations        []domain.ValuationRecord
	ExitEvents        []domain.ExitEvent
	Distributions     []domain.Distribution
	CapitalCalls      []domain.CapitalCall
	FundingRounds     []domain.CompanyFundingRound
	CompanyRisks      []domain.CompanyRisk
	InvestmentIntents []domain.InvestmentIntent
	IntentSummaries   []domain.IntentSummary
	LiquidityRequests []domain.LiquidityRequest
	CompanyUpdates    []domain.CompanyUpdate
	FinancialReports  []domain.CompanyFinancialReport
	Reports           []domain.InvestorReport
	Notifications     []domain.Notification
	RiskAlerts        []domain.RiskAlert
	RiskActions       []domain.RiskAction
	Tickets           []domain.SupportTicket
	TicketMessages    []domain.SupportTicketMessage
	PendingUsers      []domain.User
	AuditLogs         []domain.AuditLog
	Stats             map[string]int
	BuildLabel        string
	Page              int
	TotalPages        int
	PageLinks         []paginationLink
	PrevPageURL       string
	NextPageURL       string
	DashboardPages    map[string]paginationGroup
	Industries        []string
	SearchQuery       string
	SelectedIndustry  string
	Sort              string
}

type paginationLink struct {
	Label  string
	URL    string
	Active bool
}

type paginationGroup struct {
	Page        int
	TotalPages  int
	Links       []paginationLink
	PrevPageURL string
	NextPageURL string
}

func NewServer(store *store.Store) *Server {
	funcs := template.FuncMap{
		"t": i18n.T,
		"money": func(v float64) string {
			return fmt.Sprintf("$%.0f", v)
		},
		"moneyM": func(v float64) string {
			return fmt.Sprintf("$%.1fM", v)
		},
		"canAdmin":    domain.CanAccessAdmin,
		"canBuy":      domain.CanSubmitBuyInterest,
		"canSell":     domain.CanSubmitSellOrder,
		"switchLang":  switchLang,
		"statusLabel": statusLabel,
		"percent":     percent,
		"chartPoints": valuationChartPoints,
		"chartDots":   valuationChartDots,
		"chartXTicks": valuationChartXTicks,
		"chartYTicks": valuationChartYTicks,
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
	mux.HandleFunc("/notifications/read-all", s.requireAuth(s.markAllNotificationsRead))
	mux.HandleFunc("/notifications/", s.requireAuth(s.markNotificationRead))
	mux.HandleFunc("/compliance/reviews/create", s.requireAuth(s.createComplianceReview))
	mux.HandleFunc("/dashboard", s.requireAuth(s.dashboard))
	mux.HandleFunc("/companies", s.requireAuth(s.companies))
	mux.HandleFunc("/companies/", s.requireAuth(s.companyDetail))
	mux.HandleFunc("/watchlist/add", s.requireAuth(s.addWatchlist))
	mux.HandleFunc("/watchlist/remove", s.requireAuth(s.removeWatchlist))
	mux.HandleFunc("/intents/create", s.requireAuth(s.createInvestmentIntent))
	mux.HandleFunc("/market/orders", s.requireAuth(s.market))
	mux.HandleFunc("/market/liquidity", s.requireAuth(s.createLiquidityRequest))
	mux.HandleFunc("/orders/sell/cancel", s.requireAuth(s.cancelSellOrder))
	mux.HandleFunc("/orders/sell", s.requireAuth(s.createSellOrder))
	mux.HandleFunc("/orders/buy-interest/cancel", s.requireAuth(s.cancelBuyInterest))
	mux.HandleFunc("/orders/buy-interest", s.requireAuth(s.createBuyInterest))
	mux.HandleFunc("/negotiations/create", s.requireAuth(s.createNegotiation))
	mux.HandleFunc("/deals", s.requireAuth(s.deals))
	mux.HandleFunc("/deals/", s.requireAuth(s.dealActions))
	mux.HandleFunc("/portfolio", s.requireAuth(s.portfolio))
	mux.HandleFunc("/capital-calls/", s.requireAuth(s.confirmCapitalCall))
	mux.HandleFunc("/support/tickets", s.requireAuth(s.createSupportTicket))
	mux.HandleFunc("/support/tickets/reply", s.requireAuth(s.replySupportTicket))
	mux.HandleFunc("/admin", s.requireAdmin(s.admin))
	mux.HandleFunc("/admin/upgrade", s.requireAdmin(s.upgradeService))
	mux.HandleFunc("/admin/upgrade/logs", s.requireAdmin(s.upgradeLogs))
	mux.HandleFunc("/admin/companies/create", s.requireAdmin(s.createCompany))
	mux.HandleFunc("/admin/deals/create", s.requireAdmin(s.createDeal))
	mux.HandleFunc("/admin/deals/status", s.requireAdmin(s.updateDealStatus))
	mux.HandleFunc("/admin/users/risk-rating", s.requireAdmin(s.updateUserRiskRating))
	mux.HandleFunc("/admin/matches/create", s.requireAdmin(s.createMatch))
	mux.HandleFunc("/admin/documents/create", s.requireAdmin(s.createDocument))
	mux.HandleFunc("/admin/documents/", s.requireAdmin(s.advanceDocument))
	mux.HandleFunc("/admin/approvals/create", s.requireAdmin(s.createApproval))
	mux.HandleFunc("/admin/approvals/", s.requireAdmin(s.advanceApproval))
	mux.HandleFunc("/admin/escrow-payments/create", s.requireAdmin(s.createEscrowPayment))
	mux.HandleFunc("/admin/escrow-payments/", s.requireAdmin(s.advanceEscrowPayment))
	mux.HandleFunc("/admin/valuations/create", s.requireAdmin(s.createValuation))
	mux.HandleFunc("/admin/exits/create", s.requireAdmin(s.createExitEvent))
	mux.HandleFunc("/admin/company-updates/create", s.requireAdmin(s.createCompanyUpdate))
	mux.HandleFunc("/admin/distributions/create", s.requireAdmin(s.createDistribution))
	mux.HandleFunc("/admin/distributions/", s.requireAdmin(s.advanceDistribution))
	mux.HandleFunc("/admin/capital-calls/create", s.requireAdmin(s.createCapitalCall))
	mux.HandleFunc("/admin/reports/create", s.requireAdmin(s.createReport))
	mux.HandleFunc("/admin/reports/", s.requireAdmin(s.advanceReport))
	mux.HandleFunc("/admin/risks/create", s.requireAdmin(s.createRiskAlert))
	mux.HandleFunc("/admin/risks/actions/create", s.requireAdmin(s.createRiskAction))
	mux.HandleFunc("/admin/risks/", s.requireAdmin(s.resolveRiskAlert))
	mux.HandleFunc("/admin/tickets/", s.requireAdmin(s.closeTicket))
	mux.HandleFunc("/admin/reviews/", s.requireAdmin(s.approveReview))
	mux.HandleFunc("/admin/compliance-reviews/", s.requireAdmin(s.resolveComplianceReview))
	mux.HandleFunc("/admin/transactions/", s.requireAdmin(s.advanceTransaction))
	mux.HandleFunc("/admin/subscriptions/", s.requireAdmin(s.advanceSubscription))
	mux.HandleFunc("/admin/subscription-documents/create", s.requireAdmin(s.createSubscriptionDocument))
	mux.HandleFunc("/admin/subscription-documents/", s.requireAdmin(s.advanceSubscriptionDocument))
	return mux
}

type handlerFunc func(http.ResponseWriter, *http.Request, domain.User)

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data pageData) {
	if data.Lang == "" {
		data.Lang = "zh"
	}
	if data.BuildLabel == "" {
		data.BuildLabel = buildinfo.FooterLabel()
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
	portfolioValues, portfolioSummary, _ := s.store.PortfolioValuations(user.ID)
	watchlist, _ := s.store.Watchlist(user.ID)
	complianceReviews, _ := s.store.ComplianceReviews(user, 10000)
	notifications, _ := s.store.Notifications(user.ID, 10000)
	stats := map[string]int{
		"companies":    len(companies),
		"transactions": len(transactions),
		"deals":        len(deals),
		"holdings":     len(holdings),
		"watchlist":    len(watchlist),
	}
	pages := map[string]paginationGroup{}
	companies, pages["companies"] = paginateDashboardItems(r, "companies_page", "dashboard-companies", companies, 20)
	watchlist, pages["watchlist"] = paginateDashboardItems(r, "watchlist_page", "dashboard-watchlist", watchlist, 20)
	complianceReviews, pages["reviews"] = paginateDashboardItems(r, "reviews_page", "dashboard-reviews", complianceReviews, 20)
	transactions, pages["transactions"] = paginateDashboardItems(r, "transactions_page", "dashboard-transactions", transactions, 20)
	portfolioValues, pages["portfolio"] = paginateDashboardItems(r, "portfolio_page", "dashboard-portfolio", portfolioValues, 20)
	notifications, pages["notifications"] = paginateDashboardItems(r, "notifications_page", "dashboard-notifications", notifications, 20)
	s.render(w, r, "dashboard.html", pageData{Title: "Dashboard", User: user, Lang: user.Language, Companies: companies, Watchlist: watchlist, ComplianceReviews: complianceReviews, Transactions: transactions, Deals: deals, Holdings: holdings, PortfolioValues: portfolioValues, PortfolioSummary: portfolioSummary, Notifications: notifications, Stats: stats, DashboardPages: pages, Error: r.URL.Query().Get("error")})
}

func (s *Server) createComplianceReview(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/dashboard?error=form", http.StatusSeeOther)
		return
	}
	if err := s.store.CreateComplianceReview(r.Context(), user.ID, r.FormValue("review_type"), r.FormValue("note")); err != nil {
		http.Redirect(w, r, "/dashboard?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (s *Server) markNotificationRead(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/read") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/notifications/"), "/read")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.MarkNotificationRead(r.Context(), user.ID, id)
	redirect := r.Header.Get("Referer")
	if redirect == "" {
		redirect = "/dashboard"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *Server) markAllNotificationsRead(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_ = s.store.MarkAllNotificationsRead(r.Context(), user.ID)
	redirect := r.Header.Get("Referer")
	if redirect == "" {
		redirect = "/dashboard"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *Server) companies(w http.ResponseWriter, r *http.Request, user domain.User) {
	companies, err := s.store.Companies()
	watched, _ := s.store.WatchlistMap(user.ID)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	industry := strings.TrimSpace(r.URL.Query().Get("industry"))
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort"))
	industries := companyIndustries(companies)
	companies = filterAndSortCompanies(companies, q, industry, sortBy)
	data := pageData{Title: "Companies", User: user, Lang: user.Language, Companies: companies, WatchlistMap: watched, Industries: industries, SearchQuery: q, SelectedIndustry: industry, Sort: sortBy}
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
		IPOProgress:          r.FormValue("ipo_progress"),
		InvestorStructure:    r.FormValue("investor_structure"),
		ComparableCompanies:  r.FormValue("comparable_companies"),
	}
	if company.Name == "" || company.Industry == "" || company.Valuation == "" {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	if company.HeatScore == 0 {
		company.HeatScore = 70
	}
	if company.DataConfidence == 0 {
		company.DataConfidence = 75
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
	updates, _ := s.store.CompanyUpdates(company.ID, 10)
	financialReports, _ := s.store.CompanyFinancialReports(company.ID, 9)
	valuations, _ := s.store.CompanyValuations(company.ID)
	fundingRounds, _ := s.store.CompanyFundingRounds(company.ID)
	risks, _ := s.store.CompanyRisks(company.ID)
	watched, _ := s.store.WatchlistMap(user.ID)
	s.render(w, r, "company.html", pageData{Title: company.Name, User: user, Lang: user.Language, Company: company, CompanyUpdates: updates, FinancialReports: financialReports, Valuations: valuations, FundingRounds: fundingRounds, CompanyRisks: risks, WatchlistMap: watched})
}

func (s *Server) addWatchlist(w http.ResponseWriter, r *http.Request, user domain.User) {
	s.changeWatchlist(w, r, user, true)
}

func (s *Server) removeWatchlist(w http.ResponseWriter, r *http.Request, user domain.User) {
	s.changeWatchlist(w, r, user, false)
}

func (s *Server) changeWatchlist(w http.ResponseWriter, r *http.Request, user domain.User, add bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/companies?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	if add {
		_ = s.store.AddToWatchlist(r.Context(), user.ID, companyID)
	} else {
		_ = s.store.RemoveFromWatchlist(r.Context(), user.ID, companyID)
	}
	redirect := r.Header.Get("Referer")
	if redirect == "" {
		redirect = "/companies"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *Server) createInvestmentIntent(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/companies?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	minTicket, _ := strconv.ParseFloat(r.FormValue("min_ticket"), 64)
	intent := domain.InvestmentIntent{
		CompanyID:         companyID,
		Focus:             strings.TrimSpace(r.FormValue("focus")),
		Amount:            amount,
		MinTicket:         minTicket,
		Lockup:            strings.TrimSpace(r.FormValue("lockup")),
		ProductPreference: strings.TrimSpace(r.FormValue("product_preference")),
		AcceptStructures:  strings.TrimSpace(r.FormValue("accept_structures")),
		KYCWilling:        r.FormValue("kyc_willing") == "yes",
	}
	if err := s.store.CreateInvestmentIntent(r.Context(), user.ID, intent); err != nil {
		http.Redirect(w, r, fmt.Sprintf("/companies/%d?error=%s", companyID, urlSafe(err.Error())), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/companies/%d?intent=submitted#intent-form", companyID), http.StatusSeeOther)
}

func (s *Server) market(w http.ResponseWriter, r *http.Request, user domain.User) {
	companies, _ := s.store.Companies()
	sellOrders, _ := s.store.SellOrders(user)
	buyInterests, _ := s.store.BuyInterests(user)
	transactions, _ := s.store.Transactions(user)
	negotiations, _ := s.store.Negotiations(user)
	approvals, _ := s.store.ExecutionApprovals(user)
	escrowPayments, _ := s.store.EscrowPayments(user)
	liquidityRequests, _ := s.store.LiquidityRequests(user)
	selectedCompanyID, selectedCompany := selectedCompanyFromRequest(r, companies)
	if selectedCompanyID > 0 {
		sellOrders = filterSellOrdersByCompany(sellOrders, selectedCompanyID)
		buyInterests = filterBuyInterestsByCompany(buyInterests, selectedCompanyID)
		transactions = filterTransactionsByCompany(transactions, selectedCompanyID)
		negotiations = filterNegotiationsByTransactions(negotiations, transactions)
		approvals = filterApprovalsByTransactions(approvals, transactions)
		escrowPayments = filterEscrowPaymentsByTransactions(escrowPayments, transactions)
		liquidityRequests = filterLiquidityRequestsByCompany(liquidityRequests, selectedCompanyID)
	}
	stats := map[string]int{
		"companies":    len(companies),
		"sell_orders":  len(sellOrders),
		"buy_orders":   len(buyInterests),
		"transactions": len(transactions),
	}
	openTransactions := filterOpenTransactions(transactions)
	s.render(w, r, "market.html", pageData{Title: "Market", User: user, Lang: user.Language, Companies: companies, SelectedCompany: selectedCompany, SelectedCompanyID: selectedCompanyID, SellOrders: sellOrders, BuyInterests: buyInterests, Transactions: transactions, OpenTransactions: openTransactions, Negotiations: negotiations, Approvals: approvals, EscrowPayments: escrowPayments, LiquidityRequests: liquidityRequests, Stats: stats, Error: r.URL.Query().Get("error")})
}

func (s *Server) createLiquidityRequest(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/market/orders?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	low, _ := strconv.ParseFloat(r.FormValue("share_price_low"), 64)
	high, _ := strconv.ParseFloat(r.FormValue("share_price_high"), 64)
	request := domain.LiquidityRequest{
		CompanyID:      companyID,
		Side:           strings.TrimSpace(r.FormValue("side")),
		Amount:         amount,
		SharePriceLow:  low,
		SharePriceHigh: high,
		Window:         strings.TrimSpace(r.FormValue("window")),
		Note:           strings.TrimSpace(r.FormValue("note")),
	}
	if request.Window == "" {
		request.Window = "2026-Q3 季度窗口"
	}
	if err := s.store.CreateLiquidityRequest(r.Context(), user.ID, request); err != nil {
		http.Redirect(w, r, "/market/orders?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	redirect := "/market/orders"
	if companyID > 0 {
		redirect = fmt.Sprintf("/market/orders?company_id=%d", companyID)
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
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

func (s *Server) cancelSellOrder(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !domain.CanSubmitSellOrder(user) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/market/orders?error=form", http.StatusSeeOther)
		return
	}
	orderID, _ := strconv.ParseInt(r.FormValue("order_id"), 10, 64)
	if err := s.store.CancelSellOrder(r.Context(), user.ID, orderID); err != nil {
		http.Redirect(w, r, "/market/orders?error="+urlSafe(err.Error()), http.StatusSeeOther)
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

func (s *Server) cancelBuyInterest(w http.ResponseWriter, r *http.Request, user domain.User) {
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
	interestID, _ := strconv.ParseInt(r.FormValue("interest_id"), 10, 64)
	if err := s.store.CancelBuyInterest(r.Context(), user.ID, interestID); err != nil {
		http.Redirect(w, r, "/market/orders?error="+urlSafe(err.Error()), http.StatusSeeOther)
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
	companies, _ := s.store.Companies()
	deals, _ := s.store.Deals()
	spvs, _ := s.store.SPVVehicles()
	subscriptions, _ := s.store.Subscriptions(user)
	subDocuments, _ := s.store.SubscriptionDocuments(user)
	selectedCompanyID, selectedCompany := selectedCompanyFromRequest(r, companies)
	if selectedCompanyID > 0 {
		deals = filterDealsByCompany(deals, selectedCompanyID)
		spvs = filterSPVsByDeals(spvs, deals)
		subscriptions = filterSubscriptionsByDeals(subscriptions, deals)
		subDocuments = filterSubscriptionDocumentsByDeals(subDocuments, deals)
	}
	pagedDeals, page, totalPages := paginateDeals(deals, dealPageFromRequest(r), 9)
	pageLinks := dealPageLinks(r, page, totalPages)
	spvs = filterSPVsByDeals(spvs, pagedDeals)
	subscriptions = filterSubscriptionsByDeals(subscriptions, pagedDeals)
	subDocuments = filterSubscriptionDocumentsByDeals(subDocuments, pagedDeals)
	s.render(w, r, "deals.html", pageData{Title: "Deals", User: user, Lang: user.Language, SelectedCompany: selectedCompany, SelectedCompanyID: selectedCompanyID, Deals: pagedDeals, SPVs: spvs, Subscriptions: subscriptions, SubDocuments: subDocuments, Page: page, TotalPages: totalPages, PageLinks: pageLinks, PrevPageURL: dealPageURL(r, page-1), NextPageURL: dealPageURL(r, page+1), Error: r.URL.Query().Get("error")})
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
		Eligibility:     r.FormValue("eligibility"),
		KeyRisks:        r.FormValue("key_risks"),
		PartnerName:     r.FormValue("partner_name"),
		DocumentStatus:  r.FormValue("document_status"),
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

func (s *Server) updateDealStatus(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	dealID, _ := strconv.ParseInt(r.FormValue("deal_id"), 10, 64)
	if err := s.store.UpdateDealStatus(r.Context(), user.ID, dealID, r.FormValue("status"), r.FormValue("note")); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) portfolio(w http.ResponseWriter, r *http.Request, user domain.User) {
	holdings, _ := s.store.Holdings(user.ID)
	portfolioValues, portfolioSummary, _ := s.store.PortfolioValuations(user.ID)
	transactions, _ := s.store.Transactions(user)
	negotiations, _ := s.store.Negotiations(user)
	documents, _ := s.store.ExecutionDocuments(user)
	approvals, _ := s.store.ExecutionApprovals(user)
	escrowPayments, _ := s.store.EscrowPayments(user)
	subscriptions, _ := s.store.Subscriptions(user)
	subDocuments, _ := s.store.SubscriptionDocuments(user)
	valuations, _ := s.store.Valuations()
	exitEvents, _ := s.store.ExitEvents()
	distributions, _ := s.store.Distributions(user.ID)
	capitalCalls, _ := s.store.CapitalCalls(user)
	companyUpdates, _ := s.store.PortfolioCompanyUpdates(user.ID, 10)
	reports, _ := s.store.Reports(user.ID)
	tickets, _ := s.store.SupportTickets(user.ID, false)
	ticketMessages, _ := s.store.SupportTicketMessages(user, false)
	notifications, _ := s.store.Notifications(user.ID, 8)
	s.render(w, r, "portfolio.html", pageData{Title: "Portfolio", User: user, Lang: user.Language, Holdings: holdings, PortfolioValues: portfolioValues, PortfolioSummary: portfolioSummary, Transactions: transactions, Negotiations: negotiations, Documents: documents, Approvals: approvals, EscrowPayments: escrowPayments, Subscriptions: subscriptions, SubDocuments: subDocuments, Valuations: valuations, ExitEvents: exitEvents, Distributions: distributions, CapitalCalls: capitalCalls, CompanyUpdates: companyUpdates, Reports: reports, Tickets: tickets, TicketMessages: ticketMessages, Notifications: notifications})
}

func (s *Server) confirmCapitalCall(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/confirm") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/capital-calls/"), "/confirm")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.ConfirmCapitalCall(r.Context(), user.ID, id)
	http.Redirect(w, r, "/portfolio", http.StatusSeeOther)
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

func (s *Server) replySupportTicket(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/portfolio?error=form", http.StatusSeeOther)
		return
	}
	ticketID, _ := strconv.ParseInt(r.FormValue("ticket_id"), 10, 64)
	redirect := r.FormValue("redirect")
	if redirect == "" {
		redirect = "/portfolio"
	}
	if err := s.store.CreateSupportTicketMessage(r.Context(), user, ticketID, r.FormValue("message")); err != nil {
		http.Redirect(w, r, redirect+"?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *Server) admin(w http.ResponseWriter, r *http.Request, user domain.User) {
	pending, _ := s.store.UsersPendingReview()
	users, _ := s.store.Users()
	complianceReviews, _ := s.store.ComplianceReviews(user, 30)
	companies, _ := s.store.Companies()
	sellOrders, _ := s.store.SellOrders(user)
	buyInterests, _ := s.store.BuyInterests(user)
	transactions, _ := s.store.Transactions(user)
	negotiations, _ := s.store.Negotiations(user)
	deals, _ := s.store.Deals()
	spvs, _ := s.store.SPVVehicles()
	subscriptions, _ := s.store.Subscriptions(user)
	subDocuments, _ := s.store.SubscriptionDocuments(user)
	documents, _ := s.store.ExecutionDocuments(user)
	approvals, _ := s.store.ExecutionApprovals(user)
	escrowPayments, _ := s.store.EscrowPayments(user)
	valuations, _ := s.store.Valuations()
	exitEvents, _ := s.store.ExitEvents()
	distributions, _ := s.store.Distributions(0)
	capitalCalls, _ := s.store.CapitalCalls(user)
	investmentIntents, _ := s.store.InvestmentIntents(user)
	intentSummaries, _ := s.store.IntentSummaries()
	liquidityRequests, _ := s.store.LiquidityRequests(user)
	companyUpdates, _ := s.store.CompanyUpdates(0, 20)
	reports, _ := s.store.Reports(0)
	riskAlerts, _ := s.store.RiskAlerts()
	riskActions, _ := s.store.RiskActions()
	tickets, _ := s.store.SupportTickets(user.ID, true)
	ticketMessages, _ := s.store.SupportTicketMessages(user, true)
	logs, _ := s.store.AuditLogs(20)
	s.render(w, r, "admin.html", pageData{Title: "Admin", User: user, Lang: user.Language, Users: users, Companies: companies, PendingUsers: pending, ComplianceReviews: complianceReviews, SellOrders: sellOrders, BuyInterests: buyInterests, Transactions: transactions, Negotiations: negotiations, Deals: deals, SPVs: spvs, Subscriptions: subscriptions, SubDocuments: subDocuments, Documents: documents, Approvals: approvals, EscrowPayments: escrowPayments, Valuations: valuations, ExitEvents: exitEvents, Distributions: distributions, CapitalCalls: capitalCalls, InvestmentIntents: investmentIntents, IntentSummaries: intentSummaries, LiquidityRequests: liquidityRequests, CompanyUpdates: companyUpdates, Reports: reports, RiskAlerts: riskAlerts, RiskActions: riskActions, Tickets: tickets, TicketMessages: ticketMessages, AuditLogs: logs, Error: r.URL.Query().Get("error")})
}

func (s *Server) upgradeService(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	script := os.Getenv("PREIPO_UPGRADE_SCRIPT")
	if script == "" {
		script = "/opt/Pre-IPO-Market-Platform/upgrade.sh"
	}
	if !filepath.IsAbs(script) {
		wd, err := os.Getwd()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		script = filepath.Join(wd, script)
	}
	workDir := filepath.Dir(script)
	unit := fmt.Sprintf("preipo-market-upgrade-%d", time.Now().Unix())
	var cmd *exec.Cmd
	if systemdRun, err := exec.LookPath("systemd-run"); err == nil {
		cmd = exec.Command(systemdRun, "--unit", unit, "--description", "Pre-IPO Market Platform upgrade", "--working-directory", workDir, "/usr/bin/env", "bash", script)
	} else {
		cmd = exec.Command("bash", script)
		cmd.Dir = workDir
	}
	if err := cmd.Start(); err != nil {
		http.Error(w, "upgrade start failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	if wantsJSON(r) {
		writeJSON(w, map[string]string{
			"status":  "started",
			"unit":    unit,
			"message": fmt.Sprintf("升级任务已启动：%s", unit),
		})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "升级已启动，请稍后刷新页面。日志查看：journalctl -u %s -f\n", unit)
}

func (s *Server) upgradeLogs(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	unit := r.URL.Query().Get("unit")
	if !validUpgradeUnit(unit) {
		http.Error(w, "invalid unit", http.StatusBadRequest)
		return
	}
	journalctl, err := exec.LookPath("journalctl")
	if err != nil {
		writeJSON(w, map[string]string{
			"status": "unavailable",
			"logs":   "当前环境无法读取 journalctl，升级任务已在后台运行。",
		})
		return
	}
	out, err := exec.Command(journalctl, "-u", unit, "-n", "80", "--no-pager").CombinedOutput()
	if err != nil && len(out) == 0 {
		writeJSON(w, map[string]string{
			"status": "pending",
			"logs":   "等待升级日志输出...",
		})
		return
	}
	writeJSON(w, map[string]string{
		"status": "ok",
		"logs":   strings.TrimSpace(string(out)),
	})
}

func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("async") == "1"
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func validUpgradeUnit(unit string) bool {
	if !strings.HasPrefix(unit, "preipo-market-upgrade-") {
		return false
	}
	for _, r := range unit {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return len(unit) > len("preipo-market-upgrade-")
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

func (s *Server) updateUserRiskRating(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	userID, _ := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err := s.store.UpdateUserRiskRating(r.Context(), user.ID, userID, r.FormValue("risk_rating"), r.FormValue("note")); err != nil {
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

func (s *Server) createApproval(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	transactionID, _ := strconv.ParseInt(r.FormValue("transaction_id"), 10, 64)
	approval := domain.ExecutionApproval{
		TransactionID: transactionID,
		ApprovalType:  r.FormValue("approval_type"),
		DueDate:       r.FormValue("due_date"),
		Note:          r.FormValue("note"),
	}
	if err := s.store.CreateExecutionApproval(r.Context(), user.ID, approval); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) advanceApproval(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/advance") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/approvals/"), "/advance")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.AdvanceExecutionApproval(r.Context(), user.ID, id)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createEscrowPayment(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	transactionID, _ := strconv.ParseInt(r.FormValue("transaction_id"), 10, 64)
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	payment := domain.EscrowPayment{
		TransactionID: transactionID,
		Amount:        amount,
		Status:        domain.EscrowPaymentStatus(r.FormValue("status")),
		Reference:     r.FormValue("reference"),
		Note:          r.FormValue("note"),
	}
	if err := s.store.CreateEscrowPayment(r.Context(), user.ID, payment); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) advanceEscrowPayment(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/advance") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/escrow-payments/"), "/advance")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.AdvanceEscrowPayment(r.Context(), user.ID, id)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createSubscriptionDocument(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	subscriptionID, _ := strconv.ParseInt(r.FormValue("subscription_id"), 10, 64)
	documentType := r.FormValue("document_type")
	note := r.FormValue("note")
	if err := s.store.CreateSubscriptionDocument(r.Context(), user.ID, subscriptionID, documentType, note); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) advanceSubscriptionDocument(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/advance") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/subscription-documents/"), "/advance")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.AdvanceSubscriptionDocument(r.Context(), user.ID, id)
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

func (s *Server) createCompanyUpdate(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	companyID, _ := strconv.ParseInt(r.FormValue("company_id"), 10, 64)
	update := domain.CompanyUpdate{
		CompanyID:  companyID,
		UpdateType: r.FormValue("update_type"),
		Title:      r.FormValue("title"),
		Body:       r.FormValue("body"),
	}
	if err := s.store.PublishCompanyUpdate(r.Context(), user.ID, update); err != nil {
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

func (s *Server) advanceDistribution(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/advance") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/distributions/"), "/advance")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.store.AdvanceDistribution(r.Context(), user.ID, id); err != nil {
		http.Redirect(w, r, "/admin?error="+urlSafe(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) createCapitalCall(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	userID, _ := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	dealID, _ := strconv.ParseInt(r.FormValue("deal_id"), 10, 64)
	amount, _ := strconv.ParseFloat(r.FormValue("amount"), 64)
	call := domain.CapitalCall{UserID: userID, DealID: dealID, Amount: amount, DueDate: r.FormValue("due_date"), Notice: r.FormValue("notice")}
	if err := s.store.CreateCapitalCall(r.Context(), user.ID, call); err != nil {
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

func (s *Server) advanceReport(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/advance") {
		http.NotFound(w, r)
		return
	}
	idPart := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/reports/"), "/advance")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.store.AdvanceReport(r.Context(), user.ID, id); err != nil {
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

func (s *Server) createRiskAction(w http.ResponseWriter, r *http.Request, user domain.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error=form", http.StatusSeeOther)
		return
	}
	alertID, _ := strconv.ParseInt(r.FormValue("alert_id"), 10, 64)
	assigneeID, _ := strconv.ParseInt(r.FormValue("assigned_to"), 10, 64)
	if err := s.store.AddRiskAction(r.Context(), user.ID, alertID, assigneeID, r.FormValue("action"), r.FormValue("note")); err != nil {
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

func (s *Server) resolveComplianceReview(w http.ResponseWriter, r *http.Request, user domain.User) {
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
	idPart := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/admin/compliance-reviews/"), "/approve"), "/reject")
	id, err := strconv.ParseInt(strings.Trim(idPart, "/"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = s.store.ResolveComplianceReview(r.Context(), user.ID, id, action == "approve")
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

func selectedCompanyFromRequest(r *http.Request, companies []domain.Company) (int64, domain.Company) {
	companyID, _ := strconv.ParseInt(r.URL.Query().Get("company_id"), 10, 64)
	if companyID <= 0 {
		return 0, domain.Company{}
	}
	for _, company := range companies {
		if company.ID == companyID {
			return companyID, company
		}
	}
	return 0, domain.Company{}
}

func filterSellOrdersByCompany(orders []domain.SellOrder, companyID int64) []domain.SellOrder {
	filtered := make([]domain.SellOrder, 0, len(orders))
	for _, order := range orders {
		if order.CompanyID == companyID {
			filtered = append(filtered, order)
		}
	}
	return filtered
}

func filterBuyInterestsByCompany(interests []domain.BuyInterest, companyID int64) []domain.BuyInterest {
	filtered := make([]domain.BuyInterest, 0, len(interests))
	for _, interest := range interests {
		if interest.CompanyID == companyID {
			filtered = append(filtered, interest)
		}
	}
	return filtered
}

func filterTransactionsByCompany(transactions []domain.Transaction, companyID int64) []domain.Transaction {
	filtered := make([]domain.Transaction, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction.CompanyID == companyID {
			filtered = append(filtered, transaction)
		}
	}
	return filtered
}

func filterOpenTransactions(transactions []domain.Transaction) []domain.Transaction {
	filtered := make([]domain.Transaction, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction.Stage != domain.StageSettled && transaction.Stage != domain.StageCancelled {
			filtered = append(filtered, transaction)
		}
	}
	return filtered
}

func filterNegotiationsByTransactions(negotiations []domain.Negotiation, transactions []domain.Transaction) []domain.Negotiation {
	allowed := transactionIDSet(transactions)
	filtered := make([]domain.Negotiation, 0, len(negotiations))
	for _, negotiation := range negotiations {
		if allowed[negotiation.TransactionID] {
			filtered = append(filtered, negotiation)
		}
	}
	return filtered
}

func filterApprovalsByTransactions(approvals []domain.ExecutionApproval, transactions []domain.Transaction) []domain.ExecutionApproval {
	allowed := transactionIDSet(transactions)
	filtered := make([]domain.ExecutionApproval, 0, len(approvals))
	for _, approval := range approvals {
		if allowed[approval.TransactionID] {
			filtered = append(filtered, approval)
		}
	}
	return filtered
}

func filterEscrowPaymentsByTransactions(payments []domain.EscrowPayment, transactions []domain.Transaction) []domain.EscrowPayment {
	allowed := transactionIDSet(transactions)
	filtered := make([]domain.EscrowPayment, 0, len(payments))
	for _, payment := range payments {
		if allowed[payment.TransactionID] {
			filtered = append(filtered, payment)
		}
	}
	return filtered
}

func filterLiquidityRequestsByCompany(requests []domain.LiquidityRequest, companyID int64) []domain.LiquidityRequest {
	filtered := make([]domain.LiquidityRequest, 0, len(requests))
	for _, request := range requests {
		if request.CompanyID == companyID {
			filtered = append(filtered, request)
		}
	}
	return filtered
}

func transactionIDSet(transactions []domain.Transaction) map[int64]bool {
	allowed := make(map[int64]bool, len(transactions))
	for _, transaction := range transactions {
		allowed[transaction.ID] = true
	}
	return allowed
}

func companyIndustries(companies []domain.Company) []string {
	seen := map[string]bool{}
	var industries []string
	for _, company := range companies {
		if company.Industry == "" || seen[company.Industry] {
			continue
		}
		seen[company.Industry] = true
		industries = append(industries, company.Industry)
	}
	sort.Strings(industries)
	return industries
}

func paginateDashboardItems[T any](r *http.Request, param, anchor string, items []T, perPage int) ([]T, paginationGroup) {
	if perPage <= 0 {
		perPage = 20
	}
	totalPages := (len(items) + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	page, _ := strconv.Atoi(r.URL.Query().Get(param))
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	group := paginationGroup{
		Page:        page,
		TotalPages:  totalPages,
		PrevPageURL: dashboardPageURL(r, param, page-1, anchor),
		NextPageURL: dashboardPageURL(r, param, page+1, anchor),
	}
	if totalPages > 1 {
		group.Links = dashboardPageLinks(r, param, anchor, page, totalPages)
	}
	start := (page - 1) * perPage
	if start >= len(items) {
		return nil, group
	}
	end := start + perPage
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], group
}

func dashboardPageLinks(r *http.Request, param, anchor string, currentPage, totalPages int) []paginationLink {
	links := make([]paginationLink, 0, totalPages)
	for page := 1; page <= totalPages; page++ {
		links = append(links, paginationLink{
			Label:  strconv.Itoa(page),
			URL:    dashboardPageURL(r, param, page, anchor),
			Active: page == currentPage,
		})
	}
	return links
}

func dashboardPageURL(r *http.Request, param string, page int, anchor string) string {
	if page < 1 {
		page = 1
	}
	values := r.URL.Query()
	values.Set(param, strconv.Itoa(page))
	url := "/dashboard?" + values.Encode()
	if anchor != "" {
		url += "#" + anchor
	}
	return url
}

func filterAndSortCompanies(companies []domain.Company, q, industry, sortBy string) []domain.Company {
	q = strings.ToLower(strings.TrimSpace(q))
	filtered := make([]domain.Company, 0, len(companies))
	for _, company := range companies {
		if industry != "" && company.Industry != industry {
			continue
		}
		haystack := strings.ToLower(company.Name + " " + company.Industry + " " + company.Description)
		if q != "" && !strings.Contains(haystack, q) {
			continue
		}
		filtered = append(filtered, company)
	}
	switch sortBy {
	case "name":
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	case "valuation":
		sort.SliceStable(filtered, func(i, j int) bool {
			return parseValuationBillions(filtered[i].Valuation) > parseValuationBillions(filtered[j].Valuation)
		})
	default:
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].HeatScore == filtered[j].HeatScore {
				return filtered[i].ID < filtered[j].ID
			}
			return filtered[i].HeatScore > filtered[j].HeatScore
		})
	}
	return filtered
}

func filterDealsByCompany(deals []domain.Deal, companyID int64) []domain.Deal {
	filtered := make([]domain.Deal, 0, len(deals))
	for _, deal := range deals {
		if deal.CompanyID == companyID {
			filtered = append(filtered, deal)
		}
	}
	return filtered
}

func dealPageFromRequest(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		return 1
	}
	return page
}

func paginateDeals(deals []domain.Deal, page, perPage int) ([]domain.Deal, int, int) {
	if perPage <= 0 {
		perPage = 9
	}
	totalPages := (len(deals) + perPage - 1) / perPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * perPage
	if start >= len(deals) {
		return nil, page, totalPages
	}
	end := start + perPage
	if end > len(deals) {
		end = len(deals)
	}
	return deals[start:end], page, totalPages
}

func dealPageLinks(r *http.Request, currentPage, totalPages int) []paginationLink {
	links := make([]paginationLink, 0, totalPages)
	for page := 1; page <= totalPages; page++ {
		links = append(links, paginationLink{
			Label:  strconv.Itoa(page),
			URL:    dealPageURL(r, page),
			Active: page == currentPage,
		})
	}
	return links
}

func dealPageURL(r *http.Request, page int) string {
	if page < 1 {
		page = 1
	}
	values := r.URL.Query()
	values.Set("page", strconv.Itoa(page))
	return "/deals?" + values.Encode()
}

func filterSPVsByDeals(vehicles []domain.SPVVehicle, deals []domain.Deal) []domain.SPVVehicle {
	allowed := dealIDSet(deals)
	filtered := make([]domain.SPVVehicle, 0, len(vehicles))
	for _, vehicle := range vehicles {
		if allowed[vehicle.DealID] {
			filtered = append(filtered, vehicle)
		}
	}
	return filtered
}

func filterSubscriptionsByDeals(subscriptions []domain.Subscription, deals []domain.Deal) []domain.Subscription {
	allowed := dealIDSet(deals)
	filtered := make([]domain.Subscription, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		if allowed[subscription.DealID] {
			filtered = append(filtered, subscription)
		}
	}
	return filtered
}

func filterSubscriptionDocumentsByDeals(documents []domain.SubscriptionDocument, deals []domain.Deal) []domain.SubscriptionDocument {
	allowed := dealNameSet(deals)
	filtered := make([]domain.SubscriptionDocument, 0, len(documents))
	for _, document := range documents {
		if allowed[document.DealName] {
			filtered = append(filtered, document)
		}
	}
	return filtered
}

func dealIDSet(deals []domain.Deal) map[int64]bool {
	allowed := make(map[int64]bool, len(deals))
	for _, deal := range deals {
		allowed[deal.ID] = true
	}
	return allowed
}

func dealNameSet(deals []domain.Deal) map[string]bool {
	allowed := make(map[string]bool, len(deals))
	for _, deal := range deals {
		allowed[deal.Name] = true
	}
	return allowed
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

func statusLabel(status any, lang ...string) string {
	text := fmt.Sprint(status)
	useEnglish := len(lang) > 0 && lang[0] == "en"
	if useEnglish {
		return strings.ReplaceAll(text, "_", " ")
	}
	labels := map[string]string{
		"admin":                         "管理员",
		"investor":                      "投资人",
		"seller":                        "卖方",
		"institution":                   "机构",
		"pending_review":                "待审核",
		"approved":                      "已通过",
		"rejected":                      "已拒绝",
		"low":                           "低",
		"medium":                        "中",
		"high":                          "高",
		"tradable":                      "可交易",
		"limited":                       "受限",
		"open":                          "开放",
		"closed":                        "已关闭",
		"cancelled":                     "已取消",
		"interest_submitted":            "已提交意向",
		"matched":                       "已撮合",
		"company_review":                "公司审核",
		"rofr_pending":                  "优先购买权待处理",
		"payment_pending":               "待付款",
		"settled":                       "已结算",
		"submitted":                     "已提交",
		"admin_confirmed":               "后台已确认",
		"funded":                        "已出资",
		"active":                        "生效中",
		"drafted":                       "已起草",
		"sent":                          "已发送",
		"signed":                        "已签署",
		"archived":                      "已归档",
		"void":                          "已作废",
		"not_started":                   "未开始",
		"instruction_sent":              "指令已发送",
		"released":                      "已释放",
		"pending":                       "待处理",
		"available":                     "可查看",
		"paid":                          "已支付",
		"not_due":                       "未到期",
		"monitoring":                    "监控中",
		"watchlist":                     "观察中",
		"confirmed":                     "已确认",
		"resolved":                      "已解决",
		"unread":                        "未读",
		"read":                          "已读",
		"secondary":                     "二级交易",
		"spv":                           "专项载体",
		"fund_basket":                   "基金组合",
		"direct_secondary":              "直接二级转让",
		"transfer_request":              "转让申请",
		"buyer_indication":              "买方承接意向",
		"portfolio":                     "投资组合",
		"tax":                           "税务",
		"capital_account":               "资本账户",
		"annual":                        "年报",
		"quarterly":                     "季报",
		"assigned":                      "已分配",
		"note":                          "备注",
		"mitigation":                    "风险缓释",
		"revenue":                       "收入",
		"financing":                     "融资",
		"governance":                    "治理",
		"liquidity":                     "流动性",
		"rofr":                          "优先购买权",
		"company_approval":              "公司审批",
		"all":                           "全部",
		"kyc":                           "KYC",
		"aml":                           "AML",
		"accreditation":                 "合格投资人",
		"Subscription Agreement":        "认购协议",
		"Operating Agreement":           "运营协议",
		"Risk Disclosure":               "风险揭示书",
		"W-9 / Tax Form":                "税务表格",
		"NDA":                           "保密协议",
		"SPA":                           "股份购买协议",
		"ROFR Notice":                   "优先购买权通知",
		"Transfer Instruction":          "转让指令",
		"IPO readiness":                 "IPO 准备",
		"Strategic financing":           "战略融资",
		"Financing":                     "融资",
		"Revenue":                       "收入",
		"Governance":                    "治理",
		"Liquidity":                     "流动性",
		"Tender offer":                  "要约收购",
		"Dividend":                      "分红",
		"Delaware":                      "特拉华",
		"Cayman Islands":                "开曼群岛",
		"Class A":                       "A 类",
		"Limited Partner Units":         "有限合伙份额",
		"approve_user":                  "通过用户审核",
		"reject_user":                   "拒绝用户审核",
		"update_user_risk_rating":       "更新用户风险评级",
		"create_compliance_review":      "创建合规复核",
		"approve_compliance_review":     "通过合规复核",
		"reject_compliance_review":      "拒绝合规复核",
		"create_company":                "创建公司",
		"add_watchlist":                 "加入关注",
		"remove_watchlist":              "取消关注",
		"publish_company_update":        "发布公司更新",
		"create_sell_order":             "创建卖出订单",
		"cancel_sell_order":             "取消卖出订单",
		"create_buy_interest":           "创建买入意向",
		"cancel_buy_interest":           "取消买入意向",
		"create_match":                  "创建撮合",
		"create_negotiation":            "创建议价",
		"advance_transaction":           "推进交易",
		"cancel_transaction":            "取消交易",
		"create_deal":                   "创建项目",
		"update_deal_status":            "更新项目状态",
		"create_subscription":           "创建认购",
		"advance_subscription":          "推进认购",
		"cancel_subscription":           "取消认购",
		"create_subscription_document":  "创建认购文件",
		"advance_subscription_document": "推进认购文件",
		"create_execution_document":     "创建执行文件",
		"advance_execution_document":    "推进执行文件",
		"create_execution_approval":     "创建执行审批",
		"advance_execution_approval":    "推进执行审批",
		"create_escrow_payment":         "创建托管付款",
		"advance_escrow_payment":        "推进托管付款",
		"create_valuation":              "创建估值",
		"create_exit_event":             "创建退出事件",
		"create_capital_call":           "创建资本调用",
		"confirm_capital_call":          "确认资本调用",
		"create_distribution":           "创建分配",
		"advance_distribution":          "推进分配",
		"create_report":                 "创建报告",
		"advance_report":                "推进报告",
		"mark_notification_read":        "标记通知已读",
		"mark_all_notifications_read":   "全部通知标记已读",
		"create_risk_alert":             "创建风险提示",
		"add_risk_action":               "添加风险动作",
		"resolve_risk_alert":            "解决风险提示",
		"create_support_ticket":         "创建客服工单",
		"reply_support_ticket":          "回复客服工单",
		"close_support_ticket":          "关闭客服工单",
		"seed":                          "初始化",
		"user":                          "用户",
		"compliance_review":             "合规复核",
		"company":                       "公司",
		"company_update":                "公司更新",
		"sell_order":                    "卖出订单",
		"buy_interest":                  "买入意向",
		"transaction":                   "交易",
		"negotiation":                   "议价",
		"deal":                          "项目",
		"subscription":                  "认购",
		"subscription_document":         "认购文件",
		"execution_document":            "执行文件",
		"execution_approval":            "执行审批",
		"escrow_payment":                "托管付款",
		"valuation":                     "估值",
		"exit_event":                    "退出事件",
		"capital_call":                  "资本调用",
		"distribution":                  "分配",
		"investor_report":               "投资人报告",
		"notification":                  "通知",
		"risk_alert":                    "风险提示",
		"risk_action":                   "风险动作",
		"support_ticket":                "客服工单",
		"support_ticket_message":        "工单消息",
		"system":                        "系统",
		"demo data initialized":         "演示数据已初始化",
		"Welcome to Pre-IPO MVP":        "欢迎使用 Pre-IPO 演示系统",
		"欢迎使用 Pre-IPO 演示系统":             "欢迎使用 Pre-IPO 演示系统",
		"Seller workflow ready":         "卖方流程已就绪",
		"Risk rating updated":           "风险评级已更新",
		"Compliance review submitted":   "合规复核已提交",
		"Compliance review resolved":    "合规复核已处理",
		"Company update published":      "公司更新已发布",
		"Sell order cancelled":          "卖出订单已取消",
		"Buy interest cancelled":        "买入意向已取消",
		"Transaction status updated":    "交易状态已更新",
		"Transaction cancelled":         "交易已取消",
		"Deal status updated":           "项目状态已更新",
		"Subscription status updated":   "认购状态已更新",
		"Subscription cancelled":        "认购已取消",
		"Subscription document created": "认购文件已创建",
		"Subscription document updated": "认购文件已更新",
		"Execution approval updated":    "执行审批已更新",
		"Escrow payment updated":        "托管付款已更新",
		"Capital call issued":           "资本调用已发出",
		"Capital call funded":           "资本调用已出资",
		"Distribution created":          "分配已创建",
		"Distribution status updated":   "分配状态已更新",
		"Investor report available":     "投资人报告可查看",
		"Investor report updated":       "投资人报告已更新",
		"Risk alert assigned":           "风险提示已分配",
		"Support ticket reply":          "客服工单回复",
		"Support ticket closed":         "客服工单已关闭",
		"公司已加入关注列表":                     "公司已加入关注列表",
		"公司已移出关注列表":                     "公司已移出关注列表",
		"KYC、AML 与合格投资人状态已通过":              "KYC、AML 与合格投资人状态已通过",
		"KYC、AML 与合格投资人状态已拒绝":              "KYC、AML 与合格投资人状态已拒绝",
		"状态已更新为已解决":                        "状态已更新为已解决",
		"状态已更新为已关闭":                        "状态已更新为已关闭",
		"status -> read":                   "状态已更新为已读",
		"all unread notifications -> read": "全部未读通知已更新为已读",
	}
	if label, ok := labels[text]; ok {
		return label
	}
	return strings.ReplaceAll(text, "_", " ")
}

func percent(value, total any) string {
	valueFloat := toFloat(value)
	totalFloat := toFloat(total)
	if totalFloat <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", valueFloat/totalFloat*100)
}

type chartPoint struct {
	X string
	Y string
}

type chartTick struct {
	X     string
	Y     string
	Label string
}

const (
	chartPlotLeft   = 76.0
	chartPlotTop    = 24.0
	chartPlotWidth  = 610.0
	chartPlotHeight = 200.0
	chartPlotBottom = chartPlotTop + chartPlotHeight
)

func valuationChartPoints(records []domain.ValuationRecord, metric string) string {
	dots := valuationChartDots(records, metric)
	points := make([]string, 0, len(dots))
	for _, dot := range dots {
		points = append(points, dot.X+","+dot.Y)
	}
	return strings.Join(points, " ")
}

func valuationChartDots(records []domain.ValuationRecord, metric string) []chartPoint {
	if len(records) == 0 {
		return nil
	}
	minValue, maxValue := valuationChartBounds(records, metric)
	points := make([]chartPoint, 0, len(records))
	for index, record := range records {
		x := chartPlotLeft
		if len(records) > 1 {
			x = chartPlotLeft + float64(index)*chartPlotWidth/float64(len(records)-1)
		}
		value := valuationChartValue(record, metric)
		y := chartPlotTop + ((maxValue-value)/(maxValue-minValue))*chartPlotHeight
		points = append(points, chartPoint{X: chartCoord(x), Y: chartCoord(y)})
	}
	return points
}

func valuationChartXTicks(records []domain.ValuationRecord) []chartTick {
	if len(records) == 0 {
		return nil
	}
	const tickCount = 6
	ticks := make([]chartTick, 0, tickCount)
	for index := 0; index < tickCount; index++ {
		x := chartPlotLeft + float64(index)*chartPlotWidth/float64(tickCount-1)
		recordIndex := 0
		if len(records) > 1 {
			recordIndex = (index*(len(records)-1) + (tickCount-1)/2) / (tickCount - 1)
		}
		ticks = append(ticks, chartTick{
			X:     chartCoord(x),
			Y:     chartCoord(chartPlotBottom),
			Label: records[recordIndex].AsOfDate,
		})
	}
	return ticks
}

func valuationChartYTicks(records []domain.ValuationRecord, metric string) []chartTick {
	if len(records) == 0 {
		return nil
	}
	const tickCount = 5
	minValue, maxValue := valuationChartBounds(records, metric)
	ticks := make([]chartTick, 0, tickCount)
	for index := 0; index < tickCount; index++ {
		y := chartPlotTop + float64(index)*chartPlotHeight/float64(tickCount-1)
		value := maxValue - float64(index)*(maxValue-minValue)/float64(tickCount-1)
		ticks = append(ticks, chartTick{
			X:     chartCoord(chartPlotLeft),
			Y:     chartCoord(y),
			Label: valuationChartLabel(value, metric),
		})
	}
	return ticks
}

func valuationChartBounds(records []domain.ValuationRecord, metric string) (float64, float64) {
	minValue := valuationChartValue(records[0], metric)
	maxValue := minValue
	for _, record := range records[1:] {
		value := valuationChartValue(record, metric)
		if value < minValue {
			minValue = value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	if maxValue == minValue {
		if maxValue == 0 {
			maxValue = 1
		} else {
			minValue = minValue * 0.95
			maxValue = maxValue * 1.05
		}
	}
	return minValue, maxValue
}

func valuationChartValue(record domain.ValuationRecord, metric string) float64 {
	if metric == "valuation" {
		return parseValuationBillions(record.Valuation)
	}
	return record.SharePrice
}

func valuationChartLabel(value float64, metric string) string {
	if metric == "valuation" {
		if value < 1 {
			return fmt.Sprintf("$%.0fM", value*1000)
		}
		return fmt.Sprintf("$%.1fB", value)
	}
	return fmt.Sprintf("$%.0f", value)
}

func parseValuationBillions(value string) float64 {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, ",", ""))
	cleaned = strings.TrimPrefix(cleaned, "$")
	if cleaned == "" {
		return 0
	}
	multiplier := 1.0
	suffix := strings.ToUpper(cleaned[len(cleaned)-1:])
	switch suffix {
	case "B":
		cleaned = cleaned[:len(cleaned)-1]
	case "M":
		cleaned = cleaned[:len(cleaned)-1]
		multiplier = 0.001
	case "K":
		cleaned = cleaned[:len(cleaned)-1]
		multiplier = 0.000001
	}
	parsed, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return parsed * multiplier
}

func chartCoord(value float64) string {
	return fmt.Sprintf("%.1f", value)
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
