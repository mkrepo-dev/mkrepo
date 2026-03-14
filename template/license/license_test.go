package license

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mkrepo-dev/mkrepo/internal/service"
)

func TestCorectnessOfEmbededLicenses(t *testing.T) {
	t.Parallel()

	var config service.LicensesConfig
	err := yaml.Unmarshal(LicenseConfig, &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if len(config.Licenses) == 0 {
		t.Fatal("No licenses found in config")
	}

	expectedLicenses := []service.LicenseKey{
		"agpl-3.0",
		"apache-2.0",
		"bsd-3-clause",
		"cc0-1.0",
		"gpl-3.0",
		"lgpl-3.0",
		"mit",
		"mpl-2.0",
		"unlicense",
	}

	for _, key := range expectedLicenses {
		if _, exists := config.Licenses[key]; !exists {
			t.Errorf("Expected license %q not found in config", key)
		}
	}
}

func TestLicenseSchemaValidation(t *testing.T) {
	t.Parallel()

	var config service.LicensesConfig
	err := yaml.Unmarshal(LicenseConfig, &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	tests := []struct {
		name             string
		licenseKey       service.LicenseKey
		validData        map[string]any
		invalidData      map[string]any
		shouldHaveSchema bool
	}{
		{
			name:       "MIT License - Valid",
			licenseKey: "mit",
			validData: map[string]any{
				"License": map[string]any{
					"CopyrightYear":   2024,
					"CopyrightHolder": "John Doe",
				},
				"ExtraKey": "ExtraValue",
			},
			invalidData: map[string]any{
				"License": map[string]any{
					"CopyrightYear": 2024,
				},
			},
			shouldHaveSchema: true,
		},
		{
			name:       "BSD 3-Clause - Valid",
			licenseKey: "bsd-3-clause",
			validData: map[string]any{
				"License": map[string]any{
					"CopyrightYear":   2024,
					"CopyrightHolder": "Jane Smith",
				},
			},
			invalidData: map[string]any{
				"License": map[string]any{
					"CopyrightHolder": "Jane Smith",
				},
			},
			shouldHaveSchema: true,
		},
		{
			name:             "Apache 2.0 - No Schema",
			licenseKey:       "apache-2.0",
			shouldHaveSchema: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			licenseConfig, exists := config.Licenses[tt.licenseKey]
			if !exists {
				t.Fatalf("License %q not found in config", tt.licenseKey)
			}

			if tt.shouldHaveSchema {
				if licenseConfig.Schema == nil {
					t.Fatalf("Expected license %q to have a schema", tt.licenseKey)
				}

				if tt.validData != nil {
					result := licenseConfig.Schema.Validate(tt.validData)
					if !result.IsValid() {
						t.Errorf("Valid data failed validation: %v", result.Errors)
					}
				}

				if tt.invalidData != nil {
					invalidJSON, err := json.Marshal(tt.invalidData)
					if err != nil {
						t.Fatalf("Failed to marshal invalid data: %v", err)
					}

					var invalidInstance any
					if err := json.Unmarshal(invalidJSON, &invalidInstance); err != nil {
						t.Fatalf("Failed to unmarshal invalid data: %v", err)
					}

					result := licenseConfig.Schema.Validate(invalidInstance)
					if result.IsValid() {
						t.Error("Invalid data passed validation when it should have failed")
					}
				}
			} else {
				if licenseConfig.Schema != nil {
					t.Errorf("Expected license %q to not have a schema", tt.licenseKey)
				}
			}
		})
	}
}

func TestLicenseConfigFields(t *testing.T) {
	t.Parallel()

	var config service.LicensesConfig
	err := yaml.Unmarshal(LicenseConfig, &config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	tests := []struct {
		key            service.LicenseKey
		expectedName   string
		expectedFile   string
		expectedTarget string
		expectedWith   string
	}{
		{
			key:            "mit",
			expectedName:   "MIT License",
			expectedFile:   "mit.txt",
			expectedTarget: "LICENSE",
			expectedWith:   "",
		},
		{
			key:            "gpl-3.0",
			expectedName:   "GNU General Public License v3.0",
			expectedFile:   "gpl-3.0.txt",
			expectedTarget: "COPYING",
			expectedWith:   "",
		},
		{
			key:            "lgpl-3.0",
			expectedName:   "GNU Lesser General Public License v3.0",
			expectedFile:   "lgpl-3.0.txt",
			expectedTarget: "COPYING.LESSER",
			expectedWith:   "gpl-3.0",
		},
		{
			key:            "unlicense",
			expectedName:   "The Unlicense",
			expectedFile:   "unlicense.txt",
			expectedTarget: "UNLICENSE",
			expectedWith:   "",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.key), func(t *testing.T) {
			licenseConfig, exists := config.Licenses[tt.key]
			if !exists {
				t.Fatalf("License %q not found in config", tt.key)
			}

			if licenseConfig.Name != tt.expectedName {
				t.Errorf("Name = %q, want %q", licenseConfig.Name, tt.expectedName)
			}

			if licenseConfig.File != tt.expectedFile {
				t.Errorf("File = %q, want %q", licenseConfig.File, tt.expectedFile)
			}

			if licenseConfig.TargetFile != tt.expectedTarget {
				t.Errorf("TargetFile = %q, want %q", licenseConfig.TargetFile, tt.expectedTarget)
			}
		})
	}
}
