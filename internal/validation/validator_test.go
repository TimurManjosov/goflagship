package validation

import (
	"strings"
	"testing"
)

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		wantValid   bool
		wantMessage string
	}{
		{
			name:      "valid alphanumeric",
			key:       "my_flag_123",
			wantValid: true,
		},
		{
			name:      "valid with hyphen",
			key:       "my-flag-123",
			wantValid: true,
		},
		{
			name:      "valid mixed",
			key:       "my_flag-123_test",
			wantValid: true,
		},
		{
			name:        "empty key",
			key:         "",
			wantValid:   false,
			wantMessage: "Key is required",
		},
		{
			name:        "whitespace only",
			key:         "   ",
			wantValid:   false,
			wantMessage: "Key is required",
		},
		{
			name:        "too long",
			key:         strings.Repeat("a", 65),
			wantValid:   false,
			wantMessage: "Key must not exceed 64 characters",
		},
		{
			name:      "exactly 64 chars",
			key:       strings.Repeat("a", 64),
			wantValid: true,
		},
		{
			name:        "contains spaces",
			key:         "my flag",
			wantValid:   false,
			wantMessage: "Key must contain only alphanumeric characters, underscores, and hyphens",
		},
		{
			name:        "contains @",
			key:         "banner@message",
			wantValid:   false,
			wantMessage: "Key must contain only alphanumeric characters, underscores, and hyphens",
		},
		{
			name:        "contains period",
			key:         "banner.message",
			wantValid:   false,
			wantMessage: "Key must contain only alphanumeric characters, underscores, and hyphens",
		},
		{
			name:        "contains slash",
			key:         "banner/message",
			wantValid:   false,
			wantMessage: "Key must contain only alphanumeric characters, underscores, and hyphens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateKey(tt.key)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateKey(%q) valid = %v, want %v", tt.key, result.Valid, tt.wantValid)
			}
			if !tt.wantValid {
				if msg, ok := result.Errors["key"]; !ok || msg != tt.wantMessage {
					t.Errorf("ValidateKey(%q) message = %q, want %q", tt.key, msg, tt.wantMessage)
				}
			}
		})
	}
}

func TestValidateEnv(t *testing.T) {
	tests := []struct {
		name        string
		env         string
		wantValid   bool
		wantMessage string
	}{
		{
			name:      "valid prod",
			env:       "prod",
			wantValid: true,
		},
		{
			name:      "valid staging",
			env:       "staging",
			wantValid: true,
		},
		{
			name:      "valid dev",
			env:       "dev",
			wantValid: true,
		},
		{
			name:        "empty env",
			env:         "",
			wantValid:   false,
			wantMessage: "Environment is required",
		},
		{
			name:        "too long",
			env:         strings.Repeat("a", 33),
			wantValid:   false,
			wantMessage: "Environment must not exceed 32 characters",
		},
		{
			name:      "exactly 32 chars",
			env:       strings.Repeat("a", 32),
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateEnv(tt.env)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateEnv(%q) valid = %v, want %v", tt.env, result.Valid, tt.wantValid)
			}
			if !tt.wantValid {
				if msg, ok := result.Errors["env"]; !ok || msg != tt.wantMessage {
					t.Errorf("ValidateEnv(%q) message = %q, want %q", tt.env, msg, tt.wantMessage)
				}
			}
		})
	}
}

func TestValidateRollout(t *testing.T) {
	tests := []struct {
		name        string
		rollout     int32
		wantValid   bool
		wantMessage string
	}{
		{
			name:      "zero",
			rollout:   0,
			wantValid: true,
		},
		{
			name:      "100",
			rollout:   100,
			wantValid: true,
		},
		{
			name:      "50",
			rollout:   50,
			wantValid: true,
		},
		{
			name:        "negative",
			rollout:     -1,
			wantValid:   false,
			wantMessage: "Rollout must be between 0 and 100",
		},
		{
			name:        "over 100",
			rollout:     101,
			wantValid:   false,
			wantMessage: "Rollout must be between 0 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRollout(tt.rollout)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateRollout(%d) valid = %v, want %v", tt.rollout, result.Valid, tt.wantValid)
			}
			if !tt.wantValid {
				if msg, ok := result.Errors["rollout"]; !ok || msg != tt.wantMessage {
					t.Errorf("ValidateRollout(%d) message = %q, want %q", tt.rollout, msg, tt.wantMessage)
				}
			}
		})
	}
}

func TestValidateDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantValid   bool
		wantMessage string
	}{
		{
			name:        "empty",
			description: "",
			wantValid:   true,
		},
		{
			name:        "valid description",
			description: "This is a test description",
			wantValid:   true,
		},
		{
			name:        "exactly 500 chars",
			description: strings.Repeat("a", 500),
			wantValid:   true,
		},
		{
			name:        "too long",
			description: strings.Repeat("a", 501),
			wantValid:   false,
			wantMessage: "Description must not exceed 500 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateDescription(tt.description)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateDescription() valid = %v, want %v", result.Valid, tt.wantValid)
			}
			if !tt.wantValid {
				if msg, ok := result.Errors["description"]; !ok || msg != tt.wantMessage {
					t.Errorf("ValidateDescription() message = %q, want %q", msg, tt.wantMessage)
				}
			}
		})
	}
}

func TestValidateConfigSize(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		wantValid   bool
		wantMessage string
	}{
		{
			name:       "small config",
			configJSON: `{"key": "value"}`,
			wantValid:  true,
		},
		{
			name:       "empty config",
			configJSON: "",
			wantValid:  true,
		},
		{
			name:        "too large config",
			configJSON:  strings.Repeat("a", 100*1024+1),
			wantValid:   false,
			wantMessage: "Config must not exceed 100KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateConfigSize(tt.configJSON)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateConfigSize() valid = %v, want %v", result.Valid, tt.wantValid)
			}
			if !tt.wantValid {
				if msg, ok := result.Errors["config"]; !ok || msg != tt.wantMessage {
					t.Errorf("ValidateConfigSize() message = %q, want %q", msg, tt.wantMessage)
				}
			}
		})
	}
}

func TestValidateConfigJSON(t *testing.T) {
	tests := []struct {
		name      string
		configStr string
		wantValid bool
	}{
		{
			name:      "valid JSON object",
			configStr: `{"key": "value", "count": 42}`,
			wantValid: true,
		},
		{
			name:      "empty string",
			configStr: "",
			wantValid: true,
		},
		{
			name:      "whitespace only",
			configStr: "   ",
			wantValid: true,
		},
		{
			name:      "invalid JSON",
			configStr: `{invalid json}`,
			wantValid: false,
		},
		{
			name:      "JSON array",
			configStr: `[1, 2, 3]`,
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := ValidateConfigJSON(tt.configStr)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateConfigJSON(%q) valid = %v, want %v", tt.configStr, result.Valid, tt.wantValid)
			}
		})
	}
}

func TestValidateVariants(t *testing.T) {
	tests := []struct {
		name        string
		variants    []VariantValidationParams
		wantValid   bool
		wantMessage string
	}{
		{
			name:      "empty variants",
			variants:  []VariantValidationParams{},
			wantValid: true,
		},
		{
			name: "valid variants sum to 100",
			variants: []VariantValidationParams{
				{Name: "control", Weight: 50},
				{Name: "variant", Weight: 50},
			},
			wantValid: true,
		},
		{
			name: "three variants sum to 100",
			variants: []VariantValidationParams{
				{Name: "A", Weight: 33},
				{Name: "B", Weight: 33},
				{Name: "C", Weight: 34},
			},
			wantValid: true,
		},
		{
			name: "weights dont sum to 100",
			variants: []VariantValidationParams{
				{Name: "control", Weight: 40},
				{Name: "variant", Weight: 40},
			},
			wantValid:   false,
			wantMessage: "Variant weights must sum to 100",
		},
		{
			name: "empty variant name",
			variants: []VariantValidationParams{
				{Name: "", Weight: 50},
				{Name: "variant", Weight: 50},
			},
			wantValid:   false,
			wantMessage: "Variant name cannot be empty",
		},
		{
			name: "duplicate variant names",
			variants: []VariantValidationParams{
				{Name: "control", Weight: 50},
				{Name: "control", Weight: 50},
			},
			wantValid:   false,
			wantMessage: "Duplicate variant name: control",
		},
		{
			name: "negative weight",
			variants: []VariantValidationParams{
				{Name: "control", Weight: -10},
				{Name: "variant", Weight: 110},
			},
			wantValid:   false,
			wantMessage: "Variant weight must be between 0 and 100",
		},
		{
			name: "weight over 100",
			variants: []VariantValidationParams{
				{Name: "control", Weight: 101},
			},
			wantValid:   false,
			wantMessage: "Variant weight must be between 0 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateVariants(tt.variants)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateVariants() valid = %v, want %v, errors = %v", result.Valid, tt.wantValid, result.Errors)
			}
			if !tt.wantValid {
				if msg, ok := result.Errors["variants"]; !ok || msg != tt.wantMessage {
					t.Errorf("ValidateVariants() message = %q, want %q", msg, tt.wantMessage)
				}
			}
		})
	}
}

func TestValidateFlag(t *testing.T) {
	tests := []struct {
		name          string
		params        FlagValidationParams
		wantValid     bool
		wantNumErrors int
	}{
		{
			name: "all valid",
			params: FlagValidationParams{
				Key:         "valid_key",
				Env:         "prod",
				Description: "A test flag",
				Rollout:     50,
			},
			wantValid:     true,
			wantNumErrors: 0,
		},
		{
			name: "multiple errors",
			params: FlagValidationParams{
				Key:         "",
				Env:         "",
				Description: strings.Repeat("a", 501),
				Rollout:     150,
			},
			wantValid:     false,
			wantNumErrors: 4,
		},
		{
			name: "invalid key format only",
			params: FlagValidationParams{
				Key:         "invalid@key",
				Env:         "prod",
				Description: "Test",
				Rollout:     50,
			},
			wantValid:     false,
			wantNumErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateFlag(tt.params)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateFlag() valid = %v, want %v, errors = %v", result.Valid, tt.wantValid, result.Errors)
			}
			if len(result.Errors) != tt.wantNumErrors {
				t.Errorf("ValidateFlag() num errors = %d, want %d, errors = %v", len(result.Errors), tt.wantNumErrors, result.Errors)
			}
		})
	}
}
