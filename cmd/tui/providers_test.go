package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"wolink-core/internal/models"
)

// helper functions for tests

func testModelsDir(t *testing.T) string {
	t.Helper()
	// Use the real configs/models directory relative to the project root.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Walk up to find the project root (where go.mod is).
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root")
		}
		dir = parent
	}
	return filepath.Join(dir, "configs", "models")
}

// TestListProviderFiles_ReturnsAtLeastOneProvider checks that listProviderFiles
// returns at least one provider item when example-provider.yaml exists.
func TestListProviderFiles_ReturnsAtLeastOneProvider(t *testing.T) {
	dir := testModelsDir(t)
	providers, singles, err := listProviderFiles(dir)
	assert.NoError(t, err)

	// example-provider.yaml exists and is a provider file format.
	found := false
	for _, p := range providers {
		if p.provider.ID == "example-provider" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected to find example-provider in listProviderFiles result")
	_ = singles // single model files may or may not exist; not asserting
}

// TestSaveProviderFile_RoundTripsValidYAML checks that saving a ProviderFile
// produces a YAML file that re-parses correctly without error.
func TestSaveProviderFile_RoundTripsValidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	pf := &models.ProviderFile{
		ID:       "test-provider",
		Name:     "Test Provider",
		Protocol: "openai",
		BaseURL:  "http://127.0.0.1:8080",
		APIKey:   "sk-test-key",
		Models: []models.ModelDef{
			{
				ID:    "test-model",
				Name:  "Test Model",
				Type:  "chat",
				Mode:  "passthrough",
				Route: "random",
				Model: "gpt-4",
			},
		},
	}

	savedPath, err := saveProviderFile(pf, tmpDir)
	assert.NoError(t, err)
	assert.FileExists(t, savedPath)

	// Re-read and validate.
	data, err := os.ReadFile(savedPath)
	assert.NoError(t, err)

	var parsed models.ProviderFile
	err = yaml.Unmarshal(data, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, pf.ID, parsed.ID)
	assert.Equal(t, pf.Name, parsed.Name)
	assert.Equal(t, pf.Protocol, parsed.Protocol)
	assert.Equal(t, pf.BaseURL, parsed.BaseURL)
	assert.Equal(t, pf.APIKey, parsed.APIKey)
	assert.Len(t, parsed.Models, 1)
}

// TestDeleteProviderFile_RemovesFile checks that deleteProviderFile removes
// the file from the filesystem and does not error when the file exists.
func TestDeleteProviderFile_RemovesFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-provider.yaml")

	// Create a temp file.
	err := os.WriteFile(testFile, []byte("id: test"), 0644)
	assert.NoError(t, err)
	assert.FileExists(t, testFile)

	err = deleteProviderFile(testFile)
	assert.NoError(t, err)
	assert.NoFileExists(t, testFile)
}

// TestRenderProvidersContent_ReturnsProvidersHeading checks that the
// providers tab renders a heading containing "Providers" in list state.
func TestRenderProvidersContent_ReturnsProvidersHeading(t *testing.T) {
	m := newTestModel()
	m.activeTab = tabProviders

	// Ensure the model has provider state fields initialized.
	// (They will get their zero values which is fine for this test.)
	result := renderProvidersContent(m)
	assert.Contains(t, result, "Providers")
	assert.Contains(t, result, "Enter")
}

// TestSaveProviderFile_FieldLevelRoundTrip checks that every field of a
// ProviderFile survives a save → read-back → unmarshal cycle.
func TestSaveProviderFile_FieldLevelRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	pf := &models.ProviderFile{
		ID:       "roundtrip-test",
		Name:     "Roundtrip Provider",
		Protocol: "deepseek",
		BaseURL:  "https://api.deepseek.com",
		APIKey:   "sk-roundtrip-key",
		Description: map[string]string{
			"en": "test provider",
		},
		Models: []models.ModelDef{
			{
				ID:    "rt-model",
				Name:  "RT Model",
				Type:  "chat",
				Mode:  "parsed",
				Route: "balance",
				Model: "deepseek-chat",
			},
		},
	}

	savedPath, err := saveProviderFile(pf, tmpDir)
	assert.NoError(t, err)

	data, err := os.ReadFile(savedPath)
	assert.NoError(t, err)

	var parsed models.ProviderFile
	err = yaml.Unmarshal(data, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, pf.ID, parsed.ID)
	assert.Equal(t, pf.Name, parsed.Name)
	assert.Equal(t, pf.Protocol, parsed.Protocol)
	assert.Equal(t, pf.BaseURL, parsed.BaseURL)
	assert.Equal(t, pf.APIKey, parsed.APIKey)
	assert.Equal(t, pf.Description, parsed.Description)
	assert.Len(t, parsed.Models, 1)
	if len(parsed.Models) > 0 {
		assert.Equal(t, pf.Models[0].ID, parsed.Models[0].ID)
		assert.Equal(t, pf.Models[0].Name, parsed.Models[0].Name)
		assert.Equal(t, pf.Models[0].Type, parsed.Models[0].Type)
		assert.Equal(t, pf.Models[0].Mode, parsed.Models[0].Mode)
		assert.Equal(t, pf.Models[0].Route, parsed.Models[0].Route)
		assert.Equal(t, pf.Models[0].Model, parsed.Models[0].Model)
	}
}
