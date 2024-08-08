package middleware

import (
	"testing"

	"github.com/edgeflare/pgo/pkg/util"
)

func TestExtractRoleFromClaims(t *testing.T) {
	tests := []struct {
		name      string
		claims    map[string]interface{}
		path      string
		expected  string
		expectErr bool
	}{
		{
			name: "Simple path",
			claims: map[string]interface{}{
				"role": "admin",
			},
			path:      "role",
			expected:  "admin",
			expectErr: false,
		},
		{
			name: "Nested path",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"role": "user",
				},
			},
			path:      "user.role",
			expected:  "user",
			expectErr: false,
		},
		{
			name: "Array index",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"roles": []interface{}{"admin", "user"},
				},
			},
			path:      "user.roles[0]",
			expected:  "admin",
			expectErr: false,
		},
		{
			name: "Invalid array index",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"roles": []interface{}{"admin", "user"},
				},
			},
			path:      "user.roles[2]",
			expected:  "",
			expectErr: true,
		},
		{
			name: "Initial dot in path",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"role": "admin",
				},
			},
			path:      ".user.role",
			expected:  "admin",
			expectErr: false,
		},
		{
			name: "Mixed array and nested path",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"roles": []interface{}{
						map[string]interface{}{
							"type": "admin",
						},
					},
				},
			},
			path:      "user.roles[0].type",
			expected:  "admin",
			expectErr: false,
		},
		{
			name: "Path with non-string final value",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"role": 123,
				},
			},
			path:      "user.role",
			expected:  "",
			expectErr: true,
		},
		{
			name: "Non-existent path",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"role": "user",
				},
			},
			path:      "user.nonexistent",
			expected:  "",
			expectErr: true,
		},
		{
			name: "Invalid JSON path syntax",
			claims: map[string]interface{}{
				"user": map[string]interface{}{
					"roles": []interface{}{"admin", "user"},
				},
			},
			path:      "user.roles[abc]",
			expected:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := util.Jq(tt.claims, tt.path)
			if (err != nil) != tt.expectErr {
				t.Errorf("extractRoleFromClaims() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if result != tt.expected {
				t.Errorf("extractRoleFromClaims() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
