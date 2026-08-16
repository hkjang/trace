package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Status      string    `json:"status"`
	Locale      string    `json:"locale"`
	Timezone    string    `json:"timezone"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (u User) IsAdmin() bool {
	for _, role := range u.Roles {
		if role == "administrator" {
			return true
		}
	}
	return false
}

func (u User) Can(permission string) bool {
	for _, item := range u.Permissions {
		if item == permission || item == "*" {
			return true
		}
	}
	return false
}

type OIDCSettings struct {
	Enabled       bool   `json:"enabled"`
	IssuerURL     string `json:"issuerUrl"`
	ClientID      string `json:"clientId"`
	ClientSecret  string `json:"clientSecret,omitempty"`
	Scopes        string `json:"scopes"`
	UsernameClaim string `json:"usernameClaim"`
	EmailClaim    string `json:"emailClaim"`
	DisplayClaim  string `json:"displayClaim"`
	AutoProvision bool   `json:"autoProvision"`
	BaseURL       string `json:"baseUrl"`
}

type AISettings struct {
	Enabled           bool   `json:"enabled"`
	ProviderName      string `json:"providerName"`
	BaseURL           string `json:"baseUrl"`
	Protocol          string `json:"protocol"`
	APIKey            string `json:"apiKey,omitempty"`
	Model             string `json:"model"`
	MaxOutputTokens   int    `json:"maxOutputTokens"`
	ContextWindow     int    `json:"contextWindow"`
	RequestTimeoutSec int    `json:"requestTimeoutSec"`
	SystemPrompt      string `json:"systemPrompt"`
}

type WorkflowSettings struct {
	ApprovalRequired   bool `json:"approvalRequired"`
	RequireTeamManager bool `json:"requireTeamManager"`
}

type BrandingSettings struct {
	ServiceName string `json:"serviceName"`
	Tagline     string `json:"tagline"`
}

type Decision struct {
	ID                     uuid.UUID       `json:"id"`
	OwnerID                uuid.UUID       `json:"ownerId"`
	OwnerName              string          `json:"ownerName,omitempty"`
	TeamID                 *uuid.UUID      `json:"teamId,omitempty"`
	Title                  string          `json:"title"`
	Category               string          `json:"category"`
	Decision               string          `json:"decision"`
	Reason                 string          `json:"reason"`
	Assumptions            string          `json:"assumptions"`
	InvalidationConditions string          `json:"invalidationConditions"`
	Confidence             int             `json:"confidence"`
	Status                 string          `json:"status"`
	WorkflowState          string          `json:"workflowState"`
	DecidedAt              time.Time       `json:"decidedAt"`
	ReviewAt               *time.Time      `json:"reviewAt,omitempty"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
	Version                int             `json:"version"`
	Alternatives           []Alternative   `json:"alternatives,omitempty"`
	Evidence               []Evidence      `json:"evidence,omitempty"`
	Expectations           []Expectation   `json:"expectations,omitempty"`
	Outcomes               []Outcome       `json:"outcomes,omitempty"`
	Reflections            []Reflection    `json:"reflections,omitempty"`
	Insights               []AIInsight     `json:"insights,omitempty"`
	Events                 []DecisionEvent `json:"events,omitempty"`
}

type DecisionInput struct {
	Title                  string             `json:"title"`
	Category               string             `json:"category"`
	Decision               string             `json:"decision"`
	Reason                 string             `json:"reason"`
	Assumptions            string             `json:"assumptions"`
	InvalidationConditions string             `json:"invalidationConditions"`
	Confidence             int                `json:"confidence"`
	Status                 string             `json:"status"`
	DecidedAt              time.Time          `json:"decidedAt"`
	ReviewAt               *time.Time         `json:"reviewAt"`
	TeamID                 *uuid.UUID         `json:"teamId"`
	Alternatives           []AlternativeInput `json:"alternatives"`
	Evidence               []EvidenceInput    `json:"evidence"`
	Expectation            *ExpectationInput  `json:"expectation"`
}

type DecisionPatch struct {
	Title                  *string    `json:"title"`
	Category               *string    `json:"category"`
	Decision               *string    `json:"decision"`
	Reason                 *string    `json:"reason"`
	Assumptions            *string    `json:"assumptions"`
	InvalidationConditions *string    `json:"invalidationConditions"`
	Confidence             *int       `json:"confidence"`
	Status                 *string    `json:"status"`
	ReviewAt               *time.Time `json:"reviewAt"`
	Version                int        `json:"version"`
}

type Alternative struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
}

type AlternativeInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Evidence struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Source      string     `json:"source"`
	Content     string     `json:"content"`
	Snapshot    string     `json:"snapshot,omitempty"`
	Summary     string     `json:"summary"`
	Reliability *int       `json:"reliability,omitempty"`
	Stance      string     `json:"stance"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	KnownAt     time.Time  `json:"knownAt"`
	CapturedAt  time.Time  `json:"capturedAt"`
}

type EvidenceInput struct {
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Source      string     `json:"source"`
	Content     string     `json:"content"`
	Snapshot    string     `json:"snapshot"`
	Reliability *int       `json:"reliability"`
	Stance      string     `json:"stance"`
	PublishedAt *time.Time `json:"publishedAt"`
	KnownAt     time.Time  `json:"knownAt"`
}

type Expectation struct {
	ID              uuid.UUID  `json:"id"`
	Expectation     string     `json:"expectation"`
	SuccessCriteria string     `json:"successCriteria"`
	ExpectedAt      *time.Time `json:"expectedAt,omitempty"`
	Probability     *int       `json:"probability,omitempty"`
}

type ExpectationInput struct {
	Expectation     string     `json:"expectation"`
	SuccessCriteria string     `json:"successCriteria"`
	ExpectedAt      *time.Time `json:"expectedAt"`
	Probability     *int       `json:"probability"`
}

type Outcome struct {
	ID              uuid.UUID `json:"id"`
	Result          string    `json:"result"`
	OutcomeScore    int       `json:"outcomeScore"`
	DecisionQuality *int      `json:"decisionQuality,omitempty"`
	OutcomeAt       time.Time `json:"outcomeAt"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Reflection struct {
	ID                  uuid.UUID `json:"id"`
	Reflection          string    `json:"reflection"`
	Learning            string    `json:"learning"`
	ReasoningStillSound *bool     `json:"reasoningStillSound,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

type AIInsight struct {
	ID                uuid.UUID       `json:"id"`
	InsightType       string          `json:"insightType"`
	Content           json.RawMessage `json:"content"`
	Model             string          `json:"model"`
	PromptVersion     string          `json:"promptVersion"`
	ReplayAt          *time.Time      `json:"replayAt,omitempty"`
	InputSnapshotHash string          `json:"inputSnapshotHash"`
	GeneratedAt       time.Time       `json:"generatedAt"`
}

type DecisionEvent struct {
	ID          uuid.UUID       `json:"id"`
	EventType   string          `json:"eventType"`
	Payload     json.RawMessage `json:"payload"`
	EffectiveAt time.Time       `json:"effectiveAt"`
	KnownAt     time.Time       `json:"knownAt"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type Dashboard struct {
	ActiveCount  int        `json:"activeCount"`
	WaitingCount int        `json:"waitingCount"`
	ReviewDue    int        `json:"reviewDue"`
	ClosedCount  int        `json:"closedCount"`
	Recent       []Decision `json:"recent"`
}

type ApprovalRequest struct {
	ID            uuid.UUID  `json:"id"`
	DecisionID    uuid.UUID  `json:"decisionId"`
	DecisionTitle string     `json:"decisionTitle"`
	RequesterID   uuid.UUID  `json:"requesterId"`
	RequesterName string     `json:"requesterName"`
	ReviewerID    *uuid.UUID `json:"reviewerId,omitempty"`
	State         string     `json:"state"`
	RequestNote   string     `json:"requestNote"`
	ResponseNote  string     `json:"responseNote"`
	RequestedAt   time.Time  `json:"requestedAt"`
	ResolvedAt    *time.Time `json:"resolvedAt,omitempty"`
}

type PersonalKey struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"`
	Permissions   []string   `json:"permissions"`
	Status        string     `json:"status"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	LastRotatedAt time.Time  `json:"lastRotatedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type APIToken struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type Team struct {
	ID            uuid.UUID   `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	ManagerUserID *uuid.UUID  `json:"managerUserId,omitempty"`
	ManagerName   string      `json:"managerName,omitempty"`
	MemberIDs     []uuid.UUID `json:"memberIds"`
	MemberCount   int         `json:"memberCount"`
	CreatedAt     time.Time   `json:"createdAt"`
}

type CalibrationBucket struct {
	Confidence  int     `json:"confidence"`
	Count       int     `json:"count"`
	SuccessRate float64 `json:"successRate"`
}

type Analytics struct {
	TotalDecisions    int                 `json:"totalDecisions"`
	AverageConfidence float64             `json:"averageConfidence"`
	EvidenceDepth     float64             `json:"evidenceDepth"`
	ReflectionRate    float64             `json:"reflectionRate"`
	Skill             int                 `json:"skill"`
	BadLuck           int                 `json:"badLuck"`
	GoodLuck          int                 `json:"goodLuck"`
	Mistake           int                 `json:"mistake"`
	Calibration       []CalibrationBucket `json:"calibration"`
}
