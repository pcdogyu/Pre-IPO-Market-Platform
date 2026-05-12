package domain

import (
	"errors"
	"fmt"
)

type Role string

const (
	RoleAdmin       Role = "admin"
	RoleInvestor    Role = "investor"
	RoleSeller      Role = "seller"
	RoleInstitution Role = "institution"
)

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending_review"
	ReviewApproved ReviewStatus = "approved"
	ReviewRejected ReviewStatus = "rejected"
)

type TransactionStage string

const (
	StageInterestSubmitted TransactionStage = "interest_submitted"
	StageMatched           TransactionStage = "matched"
	StageCompanyReview     TransactionStage = "company_review"
	StageROFRPending       TransactionStage = "rofr_pending"
	StagePaymentPending    TransactionStage = "payment_pending"
	StageSettled           TransactionStage = "settled"
	StageCancelled         TransactionStage = "cancelled"
)

type SubscriptionStatus string

const (
	SubscriptionSubmitted      SubscriptionStatus = "submitted"
	SubscriptionAdminConfirmed SubscriptionStatus = "admin_confirmed"
	SubscriptionFunded         SubscriptionStatus = "funded"
	SubscriptionActive         SubscriptionStatus = "active"
	SubscriptionCancelled      SubscriptionStatus = "cancelled"
)

type User struct {
	ID                  int64
	Email               string
	Name                string
	Role                Role
	Language            string
	KYCStatus           ReviewStatus
	AMLStatus           ReviewStatus
	AccreditationStatus ReviewStatus
	RiskRating          string
}

type Company struct {
	ID                   int64
	Name                 string
	Industry             string
	Valuation            string
	FundingRound         string
	SharePrice           float64
	Description          string
	TradableStatus       string
	TransferRestrictions string
}

type SellOrder struct {
	ID          int64
	SellerID    int64
	SellerName  string
	CompanyID   int64
	CompanyName string
	Shares      int64
	TargetPrice float64
	Status      string
}

type BuyInterest struct {
	ID           int64
	InvestorID   int64
	InvestorName string
	CompanyID    int64
	CompanyName  string
	Amount       float64
	TargetPrice  float64
	Status       string
}

type Transaction struct {
	ID                    int64
	BuyerID               int64
	BuyerName             string
	SellerID              int64
	SellerName            string
	CompanyID             int64
	CompanyName           string
	Shares                int64
	Price                 float64
	Stage                 TransactionStage
	DocumentStatus        string
	ROFRStatus            string
	CompanyApprovalStatus string
	EscrowStatus          string
}

type Deal struct {
	ID               int64
	CompanyID        int64
	CompanyName      string
	Name             string
	DealType         string
	Structure        string
	MinSubscription  float64
	TargetSize       float64
	FeeDescription   string
	Status           string
	SubscribedAmount float64
}

type Subscription struct {
	ID           int64
	InvestorID   int64
	InvestorName string
	DealID       int64
	DealName     string
	Amount       float64
	Status       SubscriptionStatus
}

type Holding struct {
	ID          int64
	UserID      int64
	CompanyName string
	SourceType  string
	Cost        float64
	Status      string
}

type SPVVehicle struct {
	ID           int64
	DealID       int64
	DealName     string
	Name         string
	Jurisdiction string
	Manager      string
	ShareClass   string
	TotalUnits   int64
	IssuedUnits  int64
}

type ExecutionDocument struct {
	ID            int64
	TransactionID int64
	DocumentType  string
	Status        string
	SignedAt      string
	Note          string
}

type ValuationRecord struct {
	ID          int64
	CompanyID   int64
	CompanyName string
	Valuation   string
	SharePrice  float64
	AsOfDate    string
}

type ExitEvent struct {
	ID           int64
	CompanyID    int64
	CompanyName  string
	EventType    string
	Description  string
	Status       string
	ExpectedDate string
}

type Distribution struct {
	ID          int64
	UserID      int64
	HoldingID   int64
	Amount      float64
	Status      string
	TaxDocument string
}

type InvestorReport struct {
	ID         int64
	UserID     int64
	ReportType string
	Title      string
	Period     string
	Status     string
}

type RiskAlert struct {
	ID        int64
	Severity  string
	Status    string
	Subject   string
	Note      string
	CreatedAt string
}

type SupportTicket struct {
	ID        int64
	UserID    int64
	UserName  string
	Status    string
	Subject   string
	Note      string
	CreatedAt string
}

type AuditLog struct {
	ID         int64
	ActorName  string
	Action     string
	ObjectType string
	ObjectID   int64
	Note       string
	CreatedAt  string
}

func CanAccessAdmin(role Role) bool {
	return role == RoleAdmin
}

func CanSubmitBuyInterest(user User) bool {
	return (user.Role == RoleInvestor || user.Role == RoleInstitution) && user.KYCStatus == ReviewApproved && user.AMLStatus == ReviewApproved && user.AccreditationStatus == ReviewApproved
}

func CanSubmitSellOrder(user User) bool {
	return user.Role == RoleSeller && user.KYCStatus == ReviewApproved
}

func NextTransactionStage(stage TransactionStage) (TransactionStage, error) {
	switch stage {
	case StageInterestSubmitted:
		return StageMatched, nil
	case StageMatched:
		return StageCompanyReview, nil
	case StageCompanyReview:
		return StageROFRPending, nil
	case StageROFRPending:
		return StagePaymentPending, nil
	case StagePaymentPending:
		return StageSettled, nil
	case StageSettled, StageCancelled:
		return stage, fmt.Errorf("transaction is already terminal: %s", stage)
	default:
		return stage, fmt.Errorf("unknown transaction stage: %s", stage)
	}
}

func NextSubscriptionStatus(status SubscriptionStatus) (SubscriptionStatus, error) {
	switch status {
	case SubscriptionSubmitted:
		return SubscriptionAdminConfirmed, nil
	case SubscriptionAdminConfirmed:
		return SubscriptionFunded, nil
	case SubscriptionFunded:
		return SubscriptionActive, nil
	case SubscriptionActive, SubscriptionCancelled:
		return status, fmt.Errorf("subscription is already terminal: %s", status)
	default:
		return status, fmt.Errorf("unknown subscription status: %s", status)
	}
}

func ValidateSubscription(amount, minimum float64) error {
	if amount <= 0 {
		return errors.New("subscription amount must be positive")
	}
	if amount < minimum {
		return fmt.Errorf("subscription amount %.2f is below minimum %.2f", amount, minimum)
	}
	return nil
}
