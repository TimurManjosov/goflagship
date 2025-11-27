package targeting

import (
	"testing"
)

func TestEvaluate_SimpleEquals(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    UserContext
		want       bool
		wantErr    bool
	}{
		{
			name:       "plan equals premium",
			expression: `{"==": [{"var": "plan"}, "premium"]}`,
			context:    UserContext{"plan": "premium"},
			want:       true,
		},
		{
			name:       "plan does not equal premium",
			expression: `{"==": [{"var": "plan"}, "premium"]}`,
			context:    UserContext{"plan": "free"},
			want:       false,
		},
		{
			name:       "missing variable",
			expression: `{"==": [{"var": "plan"}, "premium"]}`,
			context:    UserContext{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.expression, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_NumericComparisons(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    UserContext
		want       bool
	}{
		{
			name:       "age >= 18",
			expression: `{">=": [{"var": "age"}, 18]}`,
			context:    UserContext{"age": 21},
			want:       true,
		},
		{
			name:       "age < 18",
			expression: `{">=": [{"var": "age"}, 18]}`,
			context:    UserContext{"age": 16},
			want:       false,
		},
		{
			name:       "numeric equals",
			expression: `{"==": [{"var": "count"}, 10]}`,
			context:    UserContext{"count": 10},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.expression, tt.context)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_InOperator(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    UserContext
		want       bool
	}{
		{
			name:       "country in US, CA",
			expression: `{"in": [{"var": "country"}, ["US", "CA"]]}`,
			context:    UserContext{"country": "US"},
			want:       true,
		},
		{
			name:       "country not in US, CA",
			expression: `{"in": [{"var": "country"}, ["US", "CA"]]}`,
			context:    UserContext{"country": "UK"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.expression, tt.context)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_LogicalOperators(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    UserContext
		want       bool
	}{
		{
			name:       "AND: premium AND US",
			expression: `{"and": [{"==": [{"var": "plan"}, "premium"]}, {"==": [{"var": "country"}, "US"]}]}`,
			context:    UserContext{"plan": "premium", "country": "US"},
			want:       true,
		},
		{
			name:       "AND: premium but not US",
			expression: `{"and": [{"==": [{"var": "plan"}, "premium"]}, {"==": [{"var": "country"}, "US"]}]}`,
			context:    UserContext{"plan": "premium", "country": "UK"},
			want:       false,
		},
		{
			name:       "OR: premium OR beta",
			expression: `{"or": [{"==": [{"var": "plan"}, "premium"]}, {"==": [{"var": "isBeta"}, true]}]}`,
			context:    UserContext{"plan": "free", "isBeta": true},
			want:       true,
		},
		{
			name:       "OR: neither premium nor beta",
			expression: `{"or": [{"==": [{"var": "plan"}, "premium"]}, {"==": [{"var": "isBeta"}, true]}]}`,
			context:    UserContext{"plan": "free", "isBeta": false},
			want:       false,
		},
		{
			name:       "NOT: not free plan",
			expression: `{"!": {"==": [{"var": "plan"}, "free"]}}`,
			context:    UserContext{"plan": "premium"},
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.expression, tt.context)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_ComplexRules(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    UserContext
		want       bool
	}{
		{
			name: "premium in US/CA OR beta tester",
			expression: `{"or": [
				{"and": [
					{"==": [{"var": "plan"}, "premium"]},
					{"in": [{"var": "country"}, ["US", "CA"]]}
				]},
				{"==": [{"var": "isBeta"}, true]}
			]}`,
			context: UserContext{"plan": "premium", "country": "US", "isBeta": false},
			want:    true,
		},
		{
			name: "premium in US/CA OR beta tester - beta case",
			expression: `{"or": [
				{"and": [
					{"==": [{"var": "plan"}, "premium"]},
					{"in": [{"var": "country"}, ["US", "CA"]]}
				]},
				{"==": [{"var": "isBeta"}, true]}
			]}`,
			context: UserContext{"plan": "free", "country": "UK", "isBeta": true},
			want:    true,
		},
		{
			name: "premium in US/CA OR beta tester - neither",
			expression: `{"or": [
				{"and": [
					{"==": [{"var": "plan"}, "premium"]},
					{"in": [{"var": "country"}, ["US", "CA"]]}
				]},
				{"==": [{"var": "isBeta"}, true]}
			]}`,
			context: UserContext{"plan": "free", "country": "UK", "isBeta": false},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Evaluate(tt.expression, tt.context)
			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluate_Errors(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		context    UserContext
		wantErr    error
	}{
		{
			name:       "empty expression",
			expression: "",
			context:    UserContext{},
			wantErr:    ErrEmptyExpression,
		},
		{
			name:       "whitespace only",
			expression: "   ",
			context:    UserContext{},
			wantErr:    ErrEmptyExpression,
		},
		{
			name:       "invalid JSON",
			expression: "not valid json",
			context:    UserContext{},
			wantErr:    ErrInvalidExpression,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Evaluate(tt.expression, tt.context)
			if err == nil {
				t.Errorf("Evaluate() expected error, got nil")
				return
			}
			if err != tt.wantErr {
				t.Errorf("Evaluate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateExpression(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "valid simple expression",
			expression: `{"==": [{"var": "plan"}, "premium"]}`,
			wantErr:    false,
		},
		{
			name:       "valid complex expression",
			expression: `{"and": [{"==": [{"var": "plan"}, "premium"]}, {"==": [{"var": "country"}, "US"]}]}`,
			wantErr:    false,
		},
		{
			name:       "valid boolean",
			expression: `true`,
			wantErr:    false,
		},
		{
			name:       "empty",
			expression: "",
			wantErr:    true,
		},
		{
			name:       "invalid JSON",
			expression: "not json",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExpression(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExpression() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"nil", nil, false},
		{"true", true, true},
		{"false", false, false},
		{"zero", float64(0), false},
		{"non-zero", float64(42), true},
		{"empty string", "", false},
		{"non-empty string", "hello", true},
		{"empty array", []any{}, false},
		{"non-empty array", []any{1, 2}, true},
		{"empty map", map[string]any{}, false},
		{"non-empty map", map[string]any{"key": "value"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTruthy(tt.value); got != tt.want {
				t.Errorf("isTruthy() = %v, want %v", got, tt.want)
			}
		})
	}
}
