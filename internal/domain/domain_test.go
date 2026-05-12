package domain

import "testing"

func TestRolePermissions(t *testing.T) {
	investor := User{Role: RoleInvestor, KYCStatus: ReviewApproved, AMLStatus: ReviewApproved, AccreditationStatus: ReviewApproved}
	seller := User{Role: RoleSeller, KYCStatus: ReviewApproved, AMLStatus: ReviewApproved, AccreditationStatus: ReviewPending}
	admin := User{Role: RoleAdmin}

	if !CanSubmitBuyInterest(investor) {
		t.Fatal("approved investor should submit buy interest")
	}
	if CanSubmitBuyInterest(seller) {
		t.Fatal("seller should not submit buy interest")
	}
	if !CanSubmitSellOrder(seller) {
		t.Fatal("approved seller should submit sell order")
	}
	if !CanAccessAdmin(admin.Role) {
		t.Fatal("admin should access admin area")
	}
}

func TestTransactionStageFlow(t *testing.T) {
	stage := StageInterestSubmitted
	for _, want := range []TransactionStage{StageMatched, StageCompanyReview, StageROFRPending, StagePaymentPending, StageSettled} {
		next, err := NextTransactionStage(stage)
		if err != nil {
			t.Fatalf("advance %s: %v", stage, err)
		}
		if next != want {
			t.Fatalf("got %s, want %s", next, want)
		}
		stage = next
	}
	if _, err := NextTransactionStage(StageSettled); err == nil {
		t.Fatal("settled transaction should be terminal")
	}
}

func TestSubscriptionValidationAndFlow(t *testing.T) {
	if err := ValidateSubscription(24000, 25000); err == nil {
		t.Fatal("subscription below minimum should fail")
	}
	if err := ValidateSubscription(25000, 25000); err != nil {
		t.Fatalf("subscription at minimum should pass: %v", err)
	}

	status := SubscriptionSubmitted
	for _, want := range []SubscriptionStatus{SubscriptionAdminConfirmed, SubscriptionFunded, SubscriptionActive} {
		next, err := NextSubscriptionStatus(status)
		if err != nil {
			t.Fatalf("advance %s: %v", status, err)
		}
		if next != want {
			t.Fatalf("got %s, want %s", next, want)
		}
		status = next
	}
	if _, err := NextSubscriptionStatus(SubscriptionActive); err == nil {
		t.Fatal("active subscription should be terminal")
	}
}

func TestDocumentStatusFlow(t *testing.T) {
	status := DocumentDrafted
	for _, want := range []DocumentStatus{DocumentSent, DocumentSigned, DocumentArchived} {
		next, err := NextDocumentStatus(status)
		if err != nil {
			t.Fatalf("advance %s: %v", status, err)
		}
		if next != want {
			t.Fatalf("got %s, want %s", next, want)
		}
		status = next
	}
	if _, err := NextDocumentStatus(DocumentArchived); err == nil {
		t.Fatal("archived document should be terminal")
	}
}
