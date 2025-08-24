package config

import (
	"os"
	"strings"
)

// Config holds application configuration
type Config struct {
	GitLab   GitLabConfig
	Server   ServerConfig
	Webhook  WebhookConfig
	Comments CommentsConfig
	Rules    RulesConfig
	Approval ApprovalConfig
}

// GitLabConfig holds GitLab API configuration
type GitLabConfig struct {
	BaseURL     string
	Token       string
	InsecureTLS bool   // Skip TLS certificate verification
	CACertPath  string // Path to custom CA certificate file
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
}

// WebhookConfig holds webhook security configuration
type WebhookConfig struct {
	Secret     string   // GitLab webhook secret token
	AllowedIPs []string // Optional: restrict webhook calls to specific IPs
}

// CommentsConfig holds MR comments and messages configuration
type CommentsConfig struct {
	EnableMRComments       bool   // Enable/disable MR commenting
	CommentVerbosity       string // Comment verbosity level (basic, detailed, debug)
	UpdateExistingComments bool   // Update existing comments instead of creating new ones
}

// RulesConfig holds rule-specific configuration
type RulesConfig struct {
	EnabledRules       []string // List of enabled rule names
	DisabledRules      []string // List of disabled rule names
	WarehouseRule      WarehouseRuleConfig
	ServiceAccountRule ServiceAccountRuleConfig
	MigrationsRule     MigrationsRuleConfig
	NamingRule         NamingRuleConfig
}

// WarehouseRuleConfig holds warehouse-specific configuration
type WarehouseRuleConfig struct {
	AllowTOCBypass       bool     // Allow bypassing TOC approval for specific cases
	PlatformEnvironments []string // Environments requiring platform approval
	AutoApproveEnvs      []string // Environments allowing auto-approval
}

// ServiceAccountRuleConfig holds service account validation configuration
type ServiceAccountRuleConfig struct {
	ValidateEmailFormat      bool     // Enable email format validation
	RequireIndividualEmail   bool     // Require individual vs group emails
	AllowedDomains           []string // Allowed email domains
	AstroEnvironmentsOnly    []string // Environments where Astro service accounts are allowed
	EnforceNamingConventions bool     // Enforce naming conventions
}

// MigrationsRuleConfig holds migrations validation configuration
type MigrationsRuleConfig struct {
	RequirePlatformApproval bool     // Always require platform approval
	AllowSelfServicePaths   []string // Paths that allow self-service migrations
}

// NamingRuleConfig holds naming conventions configuration
type NamingRuleConfig struct {
	ValidateTagMatching      bool // Validate data_product tag matches product name
	EnforceNamingConventions bool // Enforce naming conventions
}

// ApprovalConfig holds approval workflow configuration
type ApprovalConfig struct {
	EnableAutoApproval     bool   // Enable auto-approval functionality
	EnableTOCWorkflow      bool   // Enable TOC approval workflow
	EnablePlatformWorkflow bool   // Enable platform approval workflow
	TOCGroupID             string // GitLab group ID for TOC team
	PlatformGroupID        string // GitLab group ID for platform team
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		GitLab: GitLabConfig{
			BaseURL:     getEnv("GITLAB_BASE_URL", "https://gitlab.com"),
			Token:       getEnv("GITLAB_TOKEN", ""),
			InsecureTLS: getEnv("GITLAB_INSECURE_TLS", "false") == "true",
			CACertPath:  getEnv("GITLAB_CA_CERT_PATH", ""),
		},
		Server: ServerConfig{
			Port: getEnv("PORT", "3000"),
		},
		Webhook: WebhookConfig{
			Secret:     getEnv("WEBHOOK_SECRET", ""),
			AllowedIPs: parseIPList(getEnv("WEBHOOK_ALLOWED_IPS", "")),
		},
		Comments: CommentsConfig{
			EnableMRComments:       getEnv("ENABLE_MR_COMMENTS", "true") == "true",
			CommentVerbosity:       getEnv("COMMENT_VERBOSITY", "detailed"),
			UpdateExistingComments: getEnv("UPDATE_EXISTING_COMMENTS", "true") == "true",
		},
		Rules: RulesConfig{
			EnabledRules:  parseStringList(getEnv("ENABLED_RULES", "")),
			DisabledRules: parseStringList(getEnv("DISABLED_RULES", "")),
			WarehouseRule: WarehouseRuleConfig{
				AllowTOCBypass:       getEnv("WAREHOUSE_ALLOW_TOC_BYPASS", "false") == "true",
				PlatformEnvironments: parseStringList(getEnv("WAREHOUSE_PLATFORM_ENVS", "preprod,prod")),
				AutoApproveEnvs:      parseStringList(getEnv("WAREHOUSE_AUTO_APPROVE_ENVS", "dev,sandbox")),
			},
			ServiceAccountRule: ServiceAccountRuleConfig{
				ValidateEmailFormat:      getEnv("SA_VALIDATE_EMAIL", "true") == "true",
				RequireIndividualEmail:   getEnv("SA_REQUIRE_INDIVIDUAL_EMAIL", "true") == "true",
				AllowedDomains:           parseStringList(getEnv("SA_ALLOWED_DOMAINS", "redhat.com")),
				AstroEnvironmentsOnly:    parseStringList(getEnv("SA_ASTRO_ENVS", "preprod,prod")),
				EnforceNamingConventions: getEnv("SA_ENFORCE_NAMING", "true") == "true",
			},
			MigrationsRule: MigrationsRuleConfig{
				RequirePlatformApproval: getEnv("MIGRATIONS_REQUIRE_PLATFORM", "true") == "true",
				AllowSelfServicePaths:   parseStringList(getEnv("MIGRATIONS_SELF_SERVICE_PATHS", "")),
			},
			NamingRule: NamingRuleConfig{
				ValidateTagMatching:      getEnv("NAMING_VALIDATE_TAGS", "true") == "true",
				EnforceNamingConventions: getEnv("NAMING_ENFORCE_CONVENTIONS", "true") == "true",
			},
		},
		Approval: ApprovalConfig{
			EnableAutoApproval:     getEnv("ENABLE_AUTO_APPROVAL", "true") == "true",
			EnableTOCWorkflow:      getEnv("ENABLE_TOC_WORKFLOW", "true") == "true",
			EnablePlatformWorkflow: getEnv("ENABLE_PLATFORM_WORKFLOW", "true") == "true",
			TOCGroupID:             getEnv("TOC_GROUP_ID", ""),
			PlatformGroupID:        getEnv("PLATFORM_GROUP_ID", ""),
		},
	}
}

// HasGitLabToken returns true if GitLab token is configured
func (c *Config) HasGitLabToken() bool {
	return c.GitLab.Token != ""
}

// AnalysisMode returns a description of the current analysis mode
func (c *Config) AnalysisMode() string {
	if c.HasGitLabToken() {
		return "Full YAML analysis"
	}
	return "Limited (no GitLab token)"
}

// HasWebhookSecret returns true if webhook secret is configured
func (c *Config) HasWebhookSecret() bool {
	return c.Webhook.Secret != ""
}

// WebhookSecurityMode returns a description of the current webhook security mode
func (c *Config) WebhookSecurityMode() string {
	if c.HasWebhookSecret() {
		return "Token verification available"
	}
	return "No secret configured"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseIPList parses a comma-separated list of IP addresses
func parseIPList(ipString string) []string {
	if ipString == "" {
		return []string{}
	}
	ips := strings.Split(ipString, ",")
	result := make([]string, 0) // Initialize to empty slice, not nil
	for _, ip := range ips {
		if trimmed := strings.TrimSpace(ip); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseStringList parses a comma-separated list of strings
func parseStringList(s string) []string {
	if s == "" {
		return []string{}
	}
	items := strings.Split(s, ",")
	result := make([]string, 0) // Initialize to empty slice, not nil
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

