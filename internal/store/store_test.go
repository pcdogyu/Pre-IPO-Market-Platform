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
	user, err := s.Authenticate("investor@demo.local", "demo123")
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

func TestAdvanceTransactionCreatesSettledHolding(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
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
	admin, err := s.Authenticate("admin@demo.local", "demo123")
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

func TestCreateCompanyDealAndSupportTicket(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
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
	investor, err := s.Authenticate("investor@demo.local", "demo123")
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
}

func TestSubscriptionActivationAllocatesSPVUnits(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor@demo.local", "demo123")
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

func TestPostInvestmentAndOpsWorkflows(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
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
	if err := s.CreateReport(context.Background(), admin.ID, domain.InvestorReport{UserID: 2, ReportType: "portfolio", Title: "Q2 Report", Period: "2026-Q2", Status: "available"}); err != nil {
		t.Fatalf("create report: %v", err)
	}
	if err := s.CreateRiskAlert(context.Background(), admin.ID, domain.RiskAlert{Severity: "high", Status: "open", Subject: "Concentration limit", Note: "Review exposure"}); err != nil {
		t.Fatalf("create risk alert: %v", err)
	}
	alerts, err := s.RiskAlerts()
	if err != nil {
		t.Fatalf("risk alerts: %v", err)
	}
	if err := s.ResolveRiskAlert(context.Background(), admin.ID, alerts[0].ID); err != nil {
		t.Fatalf("resolve alert: %v", err)
	}
	if err := s.CloseSupportTicket(context.Background(), admin.ID, 1); err != nil {
		t.Fatalf("close ticket: %v", err)
	}
}

func TestRejectAndCancelWorkflows(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
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

func TestNegotiationWorkflows(t *testing.T) {
	s := testStore(t)
	investor, err := s.Authenticate("investor@demo.local", "demo123")
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
	admin, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.CreateNegotiation(context.Background(), admin, 1, 42, 800, "Admin note"); err != nil {
		t.Fatalf("admin negotiation: %v", err)
	}
}

func TestExecutionDocumentWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
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
	investor, err := s.Authenticate("investor@demo.local", "demo123")
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

func TestNotificationWorkflow(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	investor, err := s.Authenticate("investor@demo.local", "demo123")
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
}
