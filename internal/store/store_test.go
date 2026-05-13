package store

import (
	"context"
	"path/filepath"
	"testing"

	"pre-ipo-market-platform/internal/domain"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := s.SeedDemoData(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return s
}

func TestStoreCreatesOrdersAndSubscriptions(t *testing.T) {
	s := testStore(t)
	user, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateBuyInterest(context.Background(), user.ID, 1, 75000, 43.5); err != nil {
		t.Fatalf("create buy interest: %v", err)
	}
	interests, err := s.BuyInterests(user)
	if err != nil {
		t.Fatalf("list buy interests: %v", err)
	}
	if len(interests) < 2 {
		t.Fatalf("expected seeded and created buy interests, got %d", len(interests))
	}

	if err := s.CreateSubscription(context.Background(), user.ID, 1, 1000); err == nil {
		t.Fatal("subscription below minimum should fail")
	}
	if err := s.CreateSubscription(context.Background(), user.ID, 1, 30000); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	subscriptions, err := s.Subscriptions(user)
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}
	if len(subscriptions) < 2 {
		t.Fatalf("expected seeded and created subscriptions, got %d", len(subscriptions))
	}
	if err := s.CreateSubscription(context.Background(), user.ID, 1, 6000000); err == nil {
		t.Fatal("subscription above remaining capacity should fail")
	}
}

func TestAuthenticateAcceptsLegacyDemoEmailSuffix(t *testing.T) {
	s := testStore(t)
	user, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate legacy admin login: %v", err)
	}
	if user.Email != "admin" {
		t.Fatalf("legacy login resolved user %q, want %q", user.Email, "admin")
	}
}

func TestAdvanceTransactionCreatesSettledHolding(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	for i := 0; i < 4; i++ {
		if err := s.AdvanceTransaction(context.Background(), admin.ID, 1); err != nil {
			t.Fatalf("advance transaction step %d: %v", i, err)
		}
	}
	investor := domain.User{ID: 2}
	holdings, err := s.Holdings(investor.ID)
	if err != nil {
		t.Fatalf("holdings: %v", err)
	}
	var settled bool
	for _, holding := range holdings {
		if holding.CompanyName == "NeuralBridge AI" && holding.Status == string(domain.StageSettled) {
			settled = true
		}
	}
	if !settled {
		t.Fatal("expected settled holding after transaction completion")
	}
}

func TestCreateMatchedTransaction(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}

	if err := s.CreateMatchedTransaction(context.Background(), admin.ID, 1, 1, 500, 42); err != nil {
		t.Fatalf("create match: %v", err)
	}
	transactions, err := s.Transactions(admin)
	if err != nil {
		t.Fatalf("transactions: %v", err)
	}
	var found bool
	for _, tx := range transactions {
		if tx.Shares == 500 && tx.Price == 42 && tx.Stage == domain.StageMatched {
			found = true
		}
	}
	if !found {
		t.Fatal("expected matched transaction")
	}
	if err := s.CreateMatchedTransaction(context.Background(), admin.ID, 1, 1, 500, 42); err == nil {
		t.Fatal("matched order and interest should not be matched again")
	}
}

func TestUsersCanCancelOpenOrders(t *testing.T) {
	s := testStore(t)
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	seller, err := s.Authenticate("seller", "demo123")
	if err != nil {
		t.Fatalf("authenticate seller: %v", err)
	}
	if err := s.CreateSellOrder(context.Background(), seller.ID, 2, 400, 19); err != nil {
		t.Fatalf("create sell order: %v", err)
	}
	if err := s.CreateBuyInterest(context.Background(), investor.ID, 2, 25000, 19); err != nil {
		t.Fatalf("create buy interest: %v", err)
	}
	orders, err := s.SellOrders(seller)
	if err != nil {
		t.Fatalf("sell orders: %v", err)
	}
	interests, err := s.BuyInterests(investor)
	if err != nil {
		t.Fatalf("buy interests: %v", err)
	}
	var orderID int64
	for _, order := range orders {
		if order.CompanyID == 2 && order.Shares == 400 && order.Status == "open" {
			orderID = order.ID
			break
		}
	}
	var interestID int64
	for _, interest := range interests {
		if interest.CompanyID == 2 && interest.Amount == 25000 && interest.Status == string(domain.StageInterestSubmitted) {
			interestID = interest.ID
			break
		}
	}
	if orderID == 0 || interestID == 0 {
		t.Fatalf("expected open order and interest, got order=%d interest=%d", orderID, interestID)
	}
	if err := s.CancelSellOrder(context.Background(), seller.ID, orderID); err != nil {
		t.Fatalf("cancel sell order: %v", err)
	}
	if err := s.CancelBuyInterest(context.Background(), investor.ID, interestID); err != nil {
		t.Fatalf("cancel buy interest: %v", err)
	}
	if err := s.CancelSellOrder(context.Background(), seller.ID, orderID); err == nil {
		t.Fatal("cancelled sell order should not cancel again")
	}
	if err := s.CancelBuyInterest(context.Background(), investor.ID, interestID); err == nil {
		t.Fatal("cancelled buy interest should not cancel again")
	}
	orders, err = s.SellOrders(seller)
	if err != nil {
		t.Fatalf("sell orders after cancel: %v", err)
	}
	interests, err = s.BuyInterests(investor)
	if err != nil {
		t.Fatalf("buy interests after cancel: %v", err)
	}
	var foundCancelledOrder bool
	for _, order := range orders {
		if order.ID == orderID && order.Status == "cancelled" {
			foundCancelledOrder = true
		}
	}
	var foundCancelledInterest bool
	for _, interest := range interests {
		if interest.ID == interestID && interest.Status == "cancelled" {
			foundCancelledInterest = true
		}
	}
	if !foundCancelledOrder || !foundCancelledInterest {
		t.Fatal("expected cancelled order and interest")
	}
	if err := s.CancelSellOrder(context.Background(), investor.ID, 1); err == nil {
		t.Fatal("non-owner should not cancel sell order")
	}
}

func TestCreateCompanyDealAndSupportTicket(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.CreateCompany(context.Background(), admin.ID, domain.Company{
		Name:                 "Atlas Robotics",
		Industry:             "Automation",
		Valuation:            "$1.4B",
		FundingRound:         "Series C",
		SharePrice:           22.5,
		Description:          "Robotics company",
		TradableStatus:       "tradable",
		TransferRestrictions: "ROFR",
	}); err != nil {
		t.Fatalf("create company: %v", err)
	}
	companies, err := s.Companies()
	if err != nil {
		t.Fatalf("companies: %v", err)
	}
	newCompany := companies[len(companies)-1]
	if err := s.CreateDeal(context.Background(), admin.ID, domain.Deal{
		CompanyID:       newCompany.ID,
		Name:            "Atlas SPV I",
		DealType:        "spv",
		Structure:       "Single-company SPV",
		MinSubscription: 25000,
		TargetSize:      1000000,
		FeeDescription:  "2% management fee",
	}); err != nil {
		t.Fatalf("create deal: %v", err)
	}
	spvs, err := s.SPVVehicles()
	if err != nil {
		t.Fatalf("spv vehicles: %v", err)
	}
	if len(spvs) < 3 {
		t.Fatalf("expected new SPV vehicle, got %d", len(spvs))
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateSupportTicket(context.Background(), investor.ID, "Need report", "Please upload Q2 report"); err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	tickets, err := s.SupportTickets(investor.ID, false)
	if err != nil {
		t.Fatalf("tickets: %v", err)
	}
	if len(tickets) < 2 {
		t.Fatalf("expected seeded and created tickets, got %d", len(tickets))
	}
	var ticketID int64
	for _, ticket := range tickets {
		if ticket.Subject == "Need report" {
			ticketID = ticket.ID
		}
	}
	if ticketID == 0 {
		t.Fatal("expected created support ticket")
	}
	if err := s.CreateSupportTicketMessage(context.Background(), investor, ticketID, "Investor follow-up"); err != nil {
		t.Fatalf("create user ticket message: %v", err)
	}
	if err := s.CreateSupportTicketMessage(context.Background(), admin, ticketID, "Admin response"); err != nil {
		t.Fatalf("create admin ticket message: %v", err)
	}
	messages, err := s.SupportTicketMessages(investor, false)
	if err != nil {
		t.Fatalf("ticket messages: %v", err)
	}
	var foundAdminReply bool
	for _, message := range messages {
		if message.TicketID == ticketID && message.Message == "Admin response" {
			foundAdminReply = true
		}
	}
	if !foundAdminReply {
		t.Fatal("expected admin reply in ticket messages")
	}
	if err := s.CloseSupportTicket(context.Background(), admin.ID, ticketID); err != nil {
		t.Fatalf("close created ticket: %v", err)
	}
	if err := s.CreateSupportTicketMessage(context.Background(), investor, ticketID, "Late reply"); err == nil {
		t.Fatal("closed ticket should reject replies")
	}
}

func TestWatchlistWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	institution, err := s.Authenticate("institution", "demo123")
	if err != nil {
		t.Fatalf("authenticate institution: %v", err)
	}
	if err := s.AddToWatchlist(context.Background(), institution.ID, 3); err != nil {
		t.Fatalf("add watchlist: %v", err)
	}
	watched, err := s.WatchlistMap(institution.ID)
	if err != nil {
		t.Fatalf("watchlist map: %v", err)
	}
	if !watched[3] {
		t.Fatal("expected company in watchlist")
	}
	if err := s.PublishCompanyUpdate(context.Background(), admin.ID, domain.CompanyUpdate{
		CompanyID:  3,
		UpdateType: "liquidity",
		Title:      "QuantumPay tender watch",
		Body:       "Potential tender offer moved to watchlist.",
	}); err != nil {
		t.Fatalf("publish company update: %v", err)
	}
	notifications, err := s.Notifications(institution.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	var notified bool
	for _, notification := range notifications {
		if notification.Title == "Company update published" && notification.Body == "QuantumPay: QuantumPay tender watch" {
			notified = true
		}
	}
	if !notified {
		t.Fatal("watchlist user should receive company update notification")
	}
	if err := s.RemoveFromWatchlist(context.Background(), institution.ID, 3); err != nil {
		t.Fatalf("remove watchlist: %v", err)
	}
	watched, err = s.WatchlistMap(institution.ID)
	if err != nil {
		t.Fatalf("watchlist map after remove: %v", err)
	}
	if watched[3] {
		t.Fatal("company should be removed from watchlist")
	}
}

func TestSubscriptionActivationAllocatesSPVUnits(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateDeal(context.Background(), admin.ID, domain.Deal{
		CompanyID:       1,
		Name:            "Tiny SPV",
		DealType:        "spv",
		Structure:       "Capacity test SPV",
		MinSubscription: 1000,
		TargetSize:      1000,
		FeeDescription:  "0%",
	}); err != nil {
		t.Fatalf("create deal: %v", err)
	}
	deals, err := s.Deals()
	if err != nil {
		t.Fatalf("deals: %v", err)
	}
	var dealID int64
	for _, deal := range deals {
		if deal.Name == "Tiny SPV" {
			dealID = deal.ID
		}
	}
	if dealID == 0 {
		t.Fatal("expected tiny spv deal")
	}
	if err := s.CreateSubscription(context.Background(), investor.ID, dealID, 1000); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	subscriptions, err := s.Subscriptions(admin)
	if err != nil {
		t.Fatalf("subscriptions: %v", err)
	}
	var subscriptionID int64
	for _, subscription := range subscriptions {
		if subscription.DealName == "Tiny SPV" {
			subscriptionID = subscription.ID
		}
	}
	if subscriptionID == 0 {
		t.Fatal("expected subscription for tiny spv")
	}
	for i := 0; i < 3; i++ {
		if err := s.AdvanceSubscription(context.Background(), admin.ID, subscriptionID); err != nil {
			t.Fatalf("advance subscription step %d: %v", i, err)
		}
	}
	spvs, err := s.SPVVehicles()
	if err != nil {
		t.Fatalf("spv vehicles: %v", err)
	}
	var issuedUnits int64
	for _, spv := range spvs {
		if spv.DealName == "Tiny SPV" {
			issuedUnits = spv.IssuedUnits
		}
	}
	if issuedUnits != 10 {
		t.Fatalf("issued units got %d, want 10", issuedUnits)
	}
	closedDeal, err := s.Deal(dealID)
	if err != nil {
		t.Fatalf("deal: %v", err)
	}
	if closedDeal.Status != "closed" {
		t.Fatalf("deal status got %s, want closed", closedDeal.Status)
	}
	if err := s.CreateSubscription(context.Background(), investor.ID, dealID, 1000); err == nil {
		t.Fatal("closed deal should reject new subscriptions")
	}
}

func TestAdminCanUpdateDealStatus(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.UpdateDealStatus(context.Background(), admin.ID, 1, "paused", "bad status"); err == nil {
		t.Fatal("invalid deal status should fail")
	}
	if err := s.UpdateDealStatus(context.Background(), admin.ID, 1, "closed", "Capacity review"); err != nil {
		t.Fatalf("close deal: %v", err)
	}
	closed, err := s.Deal(1)
	if err != nil {
		t.Fatalf("deal: %v", err)
	}
	if closed.Status != "closed" {
		t.Fatalf("deal status got %s, want closed", closed.Status)
	}
	if err := s.CreateSubscription(context.Background(), investor.ID, 1, 30000); err == nil {
		t.Fatal("closed deal should reject subscriptions")
	}
	if err := s.UpdateDealStatus(context.Background(), admin.ID, 1, "open", "Reopened after allocation review"); err != nil {
		t.Fatalf("reopen deal: %v", err)
	}
	if err := s.CreateSubscription(context.Background(), investor.ID, 1, 30000); err != nil {
		t.Fatalf("create subscription after reopen: %v", err)
	}
	if err := s.UpdateDealStatus(context.Background(), admin.ID, 1, "cancelled", "Issuer paused allocation"); err != nil {
		t.Fatalf("cancel deal: %v", err)
	}
	subscriptions, err := s.Subscriptions(admin)
	if err != nil {
		t.Fatalf("subscriptions: %v", err)
	}
	var cancelled bool
	for _, subscription := range subscriptions {
		if subscription.DealID == 1 && subscription.Status == domain.SubscriptionCancelled {
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatal("expected pending subscriptions cancelled with deal")
	}
	if err := s.UpdateDealStatus(context.Background(), admin.ID, 1, "open", "reopen cancelled"); err == nil {
		t.Fatal("cancelled deal should not reopen")
	}
	notifications, err := s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title == "Deal status updated" {
			return
		}
	}
	t.Fatal("expected deal status notification")
}

func TestPostInvestmentAndOpsWorkflows(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.CreateValuation(context.Background(), admin.ID, 1, "$5.0B", 45, "2026-06-30"); err != nil {
		t.Fatalf("create valuation: %v", err)
	}
	valuations, err := s.Valuations()
	if err != nil {
		t.Fatalf("valuations: %v", err)
	}
	if valuations[0].Valuation != "$5.0B" {
		t.Fatalf("latest valuation got %s", valuations[0].Valuation)
	}
	if err := s.CreateExitEvent(context.Background(), admin.ID, domain.ExitEvent{CompanyID: 1, EventType: "Tender offer", Description: "Company sponsored window", Status: "confirmed", ExpectedDate: "2026-Q4"}); err != nil {
		t.Fatalf("create exit event: %v", err)
	}
	if err := s.CreateDistribution(context.Background(), admin.ID, domain.Distribution{UserID: 2, Amount: 1200, Status: "pending", TaxDocument: "K-1 draft"}); err != nil {
		t.Fatalf("create distribution: %v", err)
	}
	distributions, err := s.Distributions(0)
	if err != nil {
		t.Fatalf("admin distributions: %v", err)
	}
	var distributionID int64
	for _, distribution := range distributions {
		if distribution.Amount == 1200 && distribution.UserName != "" && distribution.Status == "pending" {
			distributionID = distribution.ID
			break
		}
	}
	if distributionID == 0 {
		t.Fatal("expected pending distribution in admin queue")
	}
	if err := s.AdvanceDistribution(context.Background(), admin.ID, distributionID); err != nil {
		t.Fatalf("advance distribution: %v", err)
	}
	distributions, err = s.Distributions(2)
	if err != nil {
		t.Fatalf("investor distributions: %v", err)
	}
	var paidDistribution bool
	for _, distribution := range distributions {
		if distribution.ID == distributionID && distribution.Status == "paid" {
			paidDistribution = true
		}
	}
	if !paidDistribution {
		t.Fatal("expected distribution advanced to paid")
	}
	if err := s.AdvanceDistribution(context.Background(), admin.ID, distributionID); err == nil {
		t.Fatal("paid distribution should not advance")
	}
	if err := s.CreateReport(context.Background(), admin.ID, domain.InvestorReport{UserID: 2, ReportType: "portfolio", Title: "Q2 Report", Period: "2026-Q2", Status: "available"}); err != nil {
		t.Fatalf("create report: %v", err)
	}
	if err := s.CreateReport(context.Background(), admin.ID, domain.InvestorReport{UserID: 2, ReportType: "tax", Title: "2026 Tax Draft", Period: "2026", Status: "pending"}); err != nil {
		t.Fatalf("create pending report: %v", err)
	}
	reports, err := s.Reports(0)
	if err != nil {
		t.Fatalf("admin reports: %v", err)
	}
	var reportID int64
	for _, report := range reports {
		if report.Title == "2026 Tax Draft" && report.UserName != "" && report.Status == "pending" {
			reportID = report.ID
			break
		}
	}
	if reportID == 0 {
		t.Fatal("expected pending report in admin queue")
	}
	if err := s.AdvanceReport(context.Background(), admin.ID, reportID); err != nil {
		t.Fatalf("advance report to available: %v", err)
	}
	if err := s.AdvanceReport(context.Background(), admin.ID, reportID); err != nil {
		t.Fatalf("advance report to archived: %v", err)
	}
	if err := s.AdvanceReport(context.Background(), admin.ID, reportID); err == nil {
		t.Fatal("archived report should not advance")
	}
	if err := s.CreateRiskAlert(context.Background(), admin.ID, domain.RiskAlert{Severity: "high", Status: "open", Subject: "Concentration limit", Note: "Review exposure"}); err != nil {
		t.Fatalf("create risk alert: %v", err)
	}
	alerts, err := s.RiskAlerts()
	if err != nil {
		t.Fatalf("risk alerts: %v", err)
	}
	if err := s.AddRiskAction(context.Background(), admin.ID, alerts[0].ID, admin.ID, "assigned", "Owner assigned"); err != nil {
		t.Fatalf("add risk action: %v", err)
	}
	actions, err := s.RiskActions()
	if err != nil {
		t.Fatalf("risk actions: %v", err)
	}
	var foundAction bool
	for _, action := range actions {
		if action.AlertID == alerts[0].ID && action.Action == "assigned" {
			foundAction = true
		}
	}
	if !foundAction {
		t.Fatal("expected assigned risk action")
	}
	if err := s.ResolveRiskAlert(context.Background(), admin.ID, alerts[0].ID); err != nil {
		t.Fatalf("resolve alert: %v", err)
	}
	if err := s.CloseSupportTicket(context.Background(), admin.ID, 1); err != nil {
		t.Fatalf("close ticket: %v", err)
	}
}

func TestPortfolioValuationSummary(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateValuation(context.Background(), admin.ID, 1, "$5.0B", 45, "2026-06-30"); err != nil {
		t.Fatalf("create valuation: %v", err)
	}
	lines, summary, err := s.PortfolioValuations(investor.ID)
	if err != nil {
		t.Fatalf("portfolio valuations: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("expected portfolio valuation lines")
	}
	if summary.CurrentValue <= summary.Cost {
		t.Fatalf("expected current value above cost after price update, got cost %.2f current %.2f", summary.Cost, summary.CurrentValue)
	}
	var foundDirect bool
	for _, line := range lines {
		if line.Label == "NeuralBridge AI" && line.SourceType == "secondary" && line.CurrentValue == 36000 {
			foundDirect = true
		}
	}
	if !foundDirect {
		t.Fatal("expected direct secondary line marked at latest share price")
	}
}

func TestRejectAndCancelWorkflows(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.RejectUser(context.Background(), admin.ID, 5); err != nil {
		t.Fatalf("reject user: %v", err)
	}
	pending, err := s.UsersPendingReview()
	if err != nil {
		t.Fatalf("pending users: %v", err)
	}
	for _, user := range pending {
		if user.ID == 5 {
			t.Fatal("rejected user should not remain in pending review queue")
		}
	}
	if err := s.CancelTransaction(context.Background(), admin.ID, 1); err != nil {
		t.Fatalf("cancel transaction: %v", err)
	}
	transactions, err := s.Transactions(admin)
	if err != nil {
		t.Fatalf("transactions: %v", err)
	}
	if transactions[len(transactions)-1].Stage != domain.StageCancelled {
		t.Fatalf("transaction stage got %s, want %s", transactions[len(transactions)-1].Stage, domain.StageCancelled)
	}
	if err := s.CancelSubscription(context.Background(), admin.ID, 1); err != nil {
		t.Fatalf("cancel subscription: %v", err)
	}
	subscriptions, err := s.Subscriptions(admin)
	if err != nil {
		t.Fatalf("subscriptions: %v", err)
	}
	var cancelled bool
	for _, subscription := range subscriptions {
		if subscription.ID == 1 && subscription.Status == domain.SubscriptionCancelled {
			cancelled = true
		}
	}
	if !cancelled {
		t.Fatal("expected cancelled subscription")
	}
}

func TestComplianceReviewWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	pending, err := s.Authenticate("pending", "demo123")
	if err != nil {
		t.Fatalf("authenticate pending user: %v", err)
	}
	if err := s.CreateComplianceReview(context.Background(), pending.ID, "all", "Updated documents uploaded"); err != nil {
		t.Fatalf("create compliance review: %v", err)
	}
	if err := s.CreateComplianceReview(context.Background(), pending.ID, "bad", "Invalid"); err == nil {
		t.Fatal("invalid compliance review type should fail")
	}
	reviews, err := s.ComplianceReviews(admin, 10)
	if err != nil {
		t.Fatalf("compliance reviews: %v", err)
	}
	var reviewID int64
	for _, review := range reviews {
		if review.UserEmail == "pending" && review.ReviewType == "all" && review.Status == domain.ReviewPending {
			reviewID = review.ID
			break
		}
	}
	if reviewID == 0 {
		t.Fatal("expected pending compliance review")
	}
	if err := s.ResolveComplianceReview(context.Background(), admin.ID, reviewID, true); err != nil {
		t.Fatalf("approve compliance review: %v", err)
	}
	approved, err := s.Authenticate("pending", "demo123")
	if err != nil {
		t.Fatalf("authenticate approved user: %v", err)
	}
	if approved.KYCStatus != domain.ReviewApproved || approved.AMLStatus != domain.ReviewApproved || approved.AccreditationStatus != domain.ReviewApproved {
		t.Fatalf("expected all compliance statuses approved, got kyc=%s aml=%s acc=%s", approved.KYCStatus, approved.AMLStatus, approved.AccreditationStatus)
	}
	notifications, err := s.Notifications(pending.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title == "Compliance review resolved" {
			return
		}
	}
	t.Fatal("expected compliance review notification")
}

func TestAdminCanUpdateUserRiskRating(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.UpdateUserRiskRating(context.Background(), admin.ID, investor.ID, "bad", "invalid"); err == nil {
		t.Fatal("invalid risk rating should fail")
	}
	if err := s.UpdateUserRiskRating(context.Background(), admin.ID, investor.ID, "high", "Annual suitability review"); err != nil {
		t.Fatalf("update risk rating: %v", err)
	}
	updated, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate updated investor: %v", err)
	}
	if updated.RiskRating != "high" {
		t.Fatalf("risk rating got %s, want high", updated.RiskRating)
	}
	notifications, err := s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title == "Risk rating updated" {
			return
		}
	}
	t.Fatal("expected risk rating notification")
}

func TestNegotiationWorkflows(t *testing.T) {
	s := testStore(t)
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateNegotiation(context.Background(), investor, 1, 41.75, 800, "Buyer counter offer"); err != nil {
		t.Fatalf("create investor negotiation: %v", err)
	}
	negotiations, err := s.Negotiations(investor)
	if err != nil {
		t.Fatalf("negotiations: %v", err)
	}
	var found bool
	for _, negotiation := range negotiations {
		if negotiation.Note == "Buyer counter offer" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected investor negotiation")
	}

	outsider := domain.User{ID: 999, Role: domain.RoleInvestor}
	if err := s.CreateNegotiation(context.Background(), outsider, 1, 40, 100, "Invalid"); err == nil {
		t.Fatal("outsider should not negotiate another user's transaction")
	}
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.CreateNegotiation(context.Background(), admin, 1, 42, 800, "Admin note"); err != nil {
		t.Fatalf("admin negotiation: %v", err)
	}
}

func TestExecutionDocumentWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.CreateExecutionDocument(context.Background(), admin.ID, 1, "Transfer Instruction", "Transfer packet"); err != nil {
		t.Fatalf("create execution document: %v", err)
	}
	docs, err := s.ExecutionDocuments(admin)
	if err != nil {
		t.Fatalf("execution documents: %v", err)
	}
	var created domain.ExecutionDocument
	for _, doc := range docs {
		if doc.DocumentType == "Transfer Instruction" {
			created = doc
			break
		}
	}
	if created.ID == 0 {
		t.Fatal("expected created document")
	}
	if err := s.AdvanceExecutionDocument(context.Background(), admin.ID, created.ID); err != nil {
		t.Fatalf("advance document to sent: %v", err)
	}
	if err := s.AdvanceExecutionDocument(context.Background(), admin.ID, created.ID); err != nil {
		t.Fatalf("advance document to signed: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	investorDocs, err := s.ExecutionDocuments(investor)
	if err != nil {
		t.Fatalf("investor documents: %v", err)
	}
	if len(investorDocs) == 0 {
		t.Fatal("investor should see documents for their transaction")
	}
}

func TestExecutionApprovalWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateExecutionApproval(context.Background(), admin.ID, domain.ExecutionApproval{
		TransactionID: 1,
		ApprovalType:  "company_approval",
		DueDate:       "2026-07-15",
		Note:          "Board consent request",
	}); err != nil {
		t.Fatalf("create execution approval: %v", err)
	}
	approvals, err := s.ExecutionApprovals(investor)
	if err != nil {
		t.Fatalf("execution approvals: %v", err)
	}
	var approvalID int64
	for _, approval := range approvals {
		if approval.ApprovalType == "company_approval" && approval.Status == "pending" {
			approvalID = approval.ID
		}
	}
	if approvalID == 0 {
		t.Fatal("expected pending company approval")
	}
	if err := s.AdvanceExecutionApproval(context.Background(), admin.ID, approvalID); err != nil {
		t.Fatalf("advance execution approval: %v", err)
	}
	transactions, err := s.Transactions(admin)
	if err != nil {
		t.Fatalf("transactions: %v", err)
	}
	var synced bool
	for _, tx := range transactions {
		if tx.ID == 1 && tx.CompanyApprovalStatus == "approved" {
			synced = true
		}
	}
	if !synced {
		t.Fatal("transaction company approval status should sync")
	}
	notifications, err := s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title == "Execution approval updated" {
			return
		}
	}
	t.Fatal("expected execution approval notification")
}

func TestSubscriptionDocumentWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateSubscription(context.Background(), investor.ID, 1, 30000); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	documents, err := s.SubscriptionDocuments(investor)
	if err != nil {
		t.Fatalf("subscription documents: %v", err)
	}
	var autoGenerated bool
	for _, document := range documents {
		if document.DocumentType == "Subscription Agreement" && document.Status == domain.DocumentDrafted {
			autoGenerated = true
			break
		}
	}
	if !autoGenerated {
		t.Fatal("expected auto-generated subscription agreement")
	}
	if err := s.CreateSubscriptionDocument(context.Background(), admin.ID, 1, "Risk Disclosure", "Risk package"); err != nil {
		t.Fatalf("create subscription document: %v", err)
	}
	documents, err = s.SubscriptionDocuments(investor)
	if err != nil {
		t.Fatalf("subscription documents after create: %v", err)
	}
	var documentID int64
	for _, document := range documents {
		if document.DocumentType == "Risk Disclosure" {
			documentID = document.ID
		}
	}
	if documentID == 0 {
		t.Fatal("expected created subscription document")
	}
	if err := s.AdvanceSubscriptionDocument(context.Background(), admin.ID, documentID); err != nil {
		t.Fatalf("advance subscription document to sent: %v", err)
	}
	if err := s.AdvanceSubscriptionDocument(context.Background(), admin.ID, documentID); err != nil {
		t.Fatalf("advance subscription document to signed: %v", err)
	}
	documents, err = s.SubscriptionDocuments(investor)
	if err != nil {
		t.Fatalf("subscription documents after advance: %v", err)
	}
	for _, document := range documents {
		if document.ID == documentID && (document.Status != domain.DocumentSigned || document.SignedAt == "") {
			t.Fatalf("document got status %s signed_at %q", document.Status, document.SignedAt)
		}
	}
	notifications, err := s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title == "Subscription document updated" {
			return
		}
	}
	t.Fatal("expected subscription document notification")
}

func TestEscrowPaymentWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	payment := domain.EscrowPayment{
		TransactionID: 1,
		Amount:        33600,
		Status:        domain.EscrowInstructionSent,
		Reference:     "ESCROW-TEST-001",
		Note:          "Wire instruction test",
	}
	if err := s.CreateEscrowPayment(context.Background(), admin.ID, payment); err != nil {
		t.Fatalf("create escrow payment: %v", err)
	}
	if err := s.CreateEscrowPayment(context.Background(), admin.ID, domain.EscrowPayment{
		TransactionID: 1,
		Amount:        1,
		Status:        "invalid",
		Reference:     "ESCROW-BAD",
	}); err == nil {
		t.Fatal("invalid escrow status should fail")
	}
	payments, err := s.EscrowPayments(investor)
	if err != nil {
		t.Fatalf("escrow payments: %v", err)
	}
	var paymentID int64
	for _, item := range payments {
		if item.Reference == payment.Reference && item.Status == domain.EscrowInstructionSent {
			paymentID = item.ID
		}
	}
	if paymentID == 0 {
		t.Fatal("expected escrow payment visible to buyer")
	}
	if err := s.AdvanceEscrowPayment(context.Background(), admin.ID, paymentID); err != nil {
		t.Fatalf("advance escrow payment to funded: %v", err)
	}
	if err := s.AdvanceEscrowPayment(context.Background(), admin.ID, paymentID); err != nil {
		t.Fatalf("advance escrow payment to released: %v", err)
	}
	payments, err = s.EscrowPayments(investor)
	if err != nil {
		t.Fatalf("escrow payments after advance: %v", err)
	}
	for _, item := range payments {
		if item.ID == paymentID && (item.Status != domain.EscrowReleased || item.ReleasedAt == "") {
			t.Fatalf("escrow payment got status %s released_at %q", item.Status, item.ReleasedAt)
		}
	}
	notifications, err := s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title == "Escrow payment updated" {
			return
		}
	}
	t.Fatal("expected escrow payment notification")
}

func TestNotificationWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.AdvanceTransaction(context.Background(), admin.ID, 1); err != nil {
		t.Fatalf("advance transaction: %v", err)
	}
	notifications, err := s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	var notificationID int64
	for _, notification := range notifications {
		if notification.Title == "Transaction status updated" && notification.Status == "unread" {
			notificationID = notification.ID
			break
		}
	}
	if notificationID == 0 {
		t.Fatal("expected unread transaction notification")
	}
	if err := s.MarkNotificationRead(context.Background(), investor.ID, notificationID); err != nil {
		t.Fatalf("mark notification read: %v", err)
	}
	notifications, err = s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications after read: %v", err)
	}
	for _, notification := range notifications {
		if notification.ID == notificationID && notification.Status != "read" {
			t.Fatalf("notification status got %s, want read", notification.Status)
		}
	}
	if err := s.AdvanceTransaction(context.Background(), admin.ID, 1); err != nil {
		t.Fatalf("advance transaction again: %v", err)
	}
	if err := s.MarkAllNotificationsRead(context.Background(), investor.ID); err != nil {
		t.Fatalf("mark all notifications read: %v", err)
	}
	notifications, err = s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications after mark all: %v", err)
	}
	for _, notification := range notifications {
		if notification.Status == "unread" {
			t.Fatalf("notification %d should be read", notification.ID)
		}
	}
}

func TestCapitalCallWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateCapitalCall(context.Background(), admin.ID, domain.CapitalCall{
		UserID:  investor.ID,
		DealID:  1,
		Amount:  7500,
		DueDate: "2026-08-01",
		Notice:  "Follow-on capital call",
	}); err != nil {
		t.Fatalf("create capital call: %v", err)
	}
	calls, err := s.CapitalCalls(investor)
	if err != nil {
		t.Fatalf("capital calls: %v", err)
	}
	var callID int64
	for _, call := range calls {
		if call.Amount == 7500 && call.Status == "pending" {
			callID = call.ID
		}
	}
	if callID == 0 {
		t.Fatal("expected pending capital call")
	}
	if err := s.ConfirmCapitalCall(context.Background(), investor.ID, callID); err != nil {
		t.Fatalf("confirm capital call: %v", err)
	}
	calls, err = s.CapitalCalls(investor)
	if err != nil {
		t.Fatalf("capital calls after confirm: %v", err)
	}
	for _, call := range calls {
		if call.ID == callID && call.Status != "funded" {
			t.Fatalf("capital call status got %s, want funded", call.Status)
		}
	}
	if err := s.ConfirmCapitalCall(context.Background(), 999, callID); err == nil {
		t.Fatal("other user should not confirm capital call")
	}
}

func TestCompanyUpdateWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	update := domain.CompanyUpdate{
		CompanyID:  1,
		UpdateType: "financing",
		Title:      "NeuralBridge financing memo",
		Body:       "Board approved a financing process update for current holders.",
	}
	if err := s.PublishCompanyUpdate(context.Background(), admin.ID, update); err != nil {
		t.Fatalf("publish company update: %v", err)
	}
	updates, err := s.CompanyUpdates(1, 10)
	if err != nil {
		t.Fatalf("company updates: %v", err)
	}
	var found bool
	for _, item := range updates {
		if item.Title == update.Title && item.CompanyName == "NeuralBridge AI" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected company update on company timeline")
	}
	portfolioUpdates, err := s.PortfolioCompanyUpdates(investor.ID, 10)
	if err != nil {
		t.Fatalf("portfolio company updates: %v", err)
	}
	found = false
	for _, item := range portfolioUpdates {
		if item.Title == update.Title {
			found = true
		}
	}
	if !found {
		t.Fatal("expected holder to see company update in portfolio")
	}
	notifications, err := s.Notifications(investor.ID, 10)
	if err != nil {
		t.Fatalf("notifications: %v", err)
	}
	for _, notification := range notifications {
		if notification.Title == "Company update published" && notification.Status == "unread" {
			return
		}
	}
	t.Fatal("expected company update notification")
}
