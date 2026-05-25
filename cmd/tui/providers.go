package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"gopkg.in/yaml.v3"
	"wolink-core/cmd/tui/forms"
	"wolink-core/internal/models"
)

// ---------------------------------------------------------------------------
// State types
// ---------------------------------------------------------------------------

// providersTabState represents the current sub-state of the Providers tab.
type providersTabState int

const (
	pList        providersTabState = iota // Main provider list view
	pAddProvider                          // Adding a new provider via wizard
	pEditProvider                         // Editing an existing provider
	pAddModel                             // Adding a new single model
	pEditModel                            // Editing an existing single model
)

// providerListItem holds a parsed provider file and its path on disk.
type providerListItem struct {
	filePath string
	provider *models.ProviderFile
}

// singleModelItem holds a parsed single-model file and its path on disk.
type singleModelItem struct {
	filePath string
	model    *models.ModelConfigFile
}

// ---------------------------------------------------------------------------
// File operations
// ---------------------------------------------------------------------------

// listProviderFiles reads all .yaml/.yml files in modelsDir and returns parsed
// provider items and single-model items. Invalid files are skipped silently.
func listProviderFiles(modelsDir string) ([]providerListItem, []singleModelItem, error) {
	entries, err := os.ReadDir(modelsDir)
	if err != nil {
		return nil, nil, err
	}

	var providers []providerListItem
	var singles []singleModelItem

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(modelsDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Try ProviderFile first (has Protocol at top level).
		var pf models.ProviderFile
		if err := yaml.Unmarshal(data, &pf); err == nil && pf.Protocol != "" {
			providers = append(providers, providerListItem{filePath: filePath, provider: &pf})
			continue
		}

		// Try ModelConfigFile (has Meta.Protocol or ID).
		var mf models.ModelConfigFile
		if err := yaml.Unmarshal(data, &mf); err == nil && mf.ID != "" {
			singles = append(singles, singleModelItem{filePath: filePath, model: &mf})
			continue
		}
	}

	return providers, singles, nil
}

// saveProviderFile marshals pf to YAML, validates by re-parsing, and writes to
// modelsDir using pf.ID as the filename. Returns the full path written.
func saveProviderFile(pf *models.ProviderFile, modelsDir string) (string, error) {
	data, err := yaml.Marshal(pf)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}

	// Validate by re-parsing.
	var check models.ProviderFile
	if err := yaml.Unmarshal(data, &check); err != nil {
		return "", fmt.Errorf("save-time yaml validation failed: %w", err)
	}

	filename := pf.ID + ".yaml"
	filePath := filepath.Join(modelsDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}

	return filePath, nil
}

// saveModelConfigFile marshals mf to YAML, validates by re-parsing, and writes
// to modelsDir using mf.ID as the filename. Returns the full path written.
func saveModelConfigFile(mf *models.ModelConfigFile, modelsDir string) (string, error) {
	data, err := yaml.Marshal(mf)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}

	// Validate by re-parsing.
	var check models.ModelConfigFile
	if err := yaml.Unmarshal(data, &check); err != nil {
		return "", fmt.Errorf("save-time yaml validation failed: %w", err)
	}

	filename := mf.ID + ".yaml"
	filePath := filepath.Join(modelsDir, filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}

	return filePath, nil
}

// deleteProviderFile removes a file from the filesystem. Returns nil if the
// file doesn't exist or is successfully removed.
func deleteProviderFile(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(filePath)
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// renderProvidersContent returns the full rendered content for the Providers
// tab, dispatching to the sub-state's render function.
func renderProvidersContent(m model) string {
	switch m.providersState {
	case pList:
		return renderProvidersList(m)
	case pAddProvider, pEditProvider:
		return m.providerForm.View()
	case pAddModel, pEditModel:
		return m.modelForm.View()
	default:
		return renderProvidersList(m)
	}
}

// renderProvidersList renders the provider list view with action instructions.
func renderProvidersList(m model) string {
	var b strings.Builder

	helpText := m.styles.helpStyle.Render("Enter: edit | d: delete | a: add provider | n: add single model")
	heading := m.styles.titleStyle.Render("Providers")
	b.WriteString(fmt.Sprintf(" %s\n", heading))
	b.WriteString(fmt.Sprintf(" %s\n", helpText))
	b.WriteString(" " + strings.Repeat("─", clampWidth(m.width-2, 60)) + "\n\n")

	if len(m.providerListItems) == 0 && len(m.singleModelItems) == 0 {
		b.WriteString("   No providers or models configured.\n")
		b.WriteString("   Press 'a' to add a provider or 'n' to add a single model.\n")
		return b.String()
	}

	if len(m.providerListItems) > 0 {
		b.WriteString("   Providers:\n")
		for i, item := range m.providerListItems {
			cursor := "  "
			if i == m.selectedProviderIdx {
				cursor = " >"
			}
			modelCount := fmt.Sprintf("%d models", len(item.provider.Models))
			b.WriteString(fmt.Sprintf("   %s %-30s %-12s %s\n",
				cursor,
				item.provider.Name,
				item.provider.Protocol,
				modelCount))
		}
		b.WriteString("\n")
	}

	if len(m.singleModelItems) > 0 {
		b.WriteString("   Single Models:\n")
		for i, item := range m.singleModelItems {
			cursor := "  "
			if i == m.selectedModelIdx && len(m.providerListItems) == 0 {
				cursor = " >"
			}
			b.WriteString(fmt.Sprintf("   %s %-30s %-12s\n",
				cursor,
				item.model.Name,
				item.model.Meta.Protocol))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

// handleProvidersKeyMsg handles key events when the Providers tab is active.
// It dispatches based on the current providersTabState.
func handleProvidersKeyMsg(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	switch m.providersState {
	// -----------------------------------------------------------------------
	// Provider list state
	// -----------------------------------------------------------------------
	case pList:
		switch msg.String() {
		case "a":
			// Open the provider creation wizard.
			m.providersState = pAddProvider
			m.providerForm = forms.NewProviderForm()
			cmd := m.providerForm.Init()
			return m, cmd

		case "n":
			// Open the single model creation form.
			m.providersState = pAddModel
			m.modelForm = forms.NewModelForm()
			cmd := m.modelForm.Init()
			return m, cmd

		case "enter":
			// Edit the selected provider.
			if m.selectedProviderIdx < len(m.providerListItems) {
				item := m.providerListItems[m.selectedProviderIdx]
				m.providersState = pEditProvider
				m.providerForm = forms.NewProviderFormEdit(item.provider)
				cmd := m.providerForm.Init()
				return m, cmd
			}

		case "d":
			// Delete the selected provider file.
			if m.selectedProviderIdx < len(m.providerListItems) {
				item := m.providerListItems[m.selectedProviderIdx]
				if err := deleteProviderFile(item.filePath); err == nil {
					// Reload the list.
					providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
					m.providerListItems = providers
					m.singleModelItems = singles
					if m.selectedProviderIdx >= len(m.providerListItems) {
						m.selectedProviderIdx = max(0, len(m.providerListItems)-1)
					}
					// Set restart-required if gateway is running.
					if m.gwLifecycle.IsRunning() {
						m.restartRequired = true
					}
				}
			}
			return m, nil

		case "j", "down":
			total := len(m.providerListItems) + len(m.singleModelItems)
			if total > 0 {
				m.selectedProviderIdx++
				if m.selectedProviderIdx >= len(m.providerListItems) {
					m.selectedProviderIdx = 0
				}
			}
			return m, nil

		case "k", "up":
			total := len(m.providerListItems) + len(m.singleModelItems)
			if total > 0 {
				m.selectedProviderIdx--
				if m.selectedProviderIdx < 0 {
					m.selectedProviderIdx = len(m.providerListItems) - 1
				}
			}
			return m, nil
		}

	// -----------------------------------------------------------------------
	// Provider form states (add / edit)
	// -----------------------------------------------------------------------
	case pAddProvider, pEditProvider:
		var cmd tea.Cmd
		m.providerForm, cmd = m.providerForm.Update(msg)
		if m.providerForm.Finished() {
			// Save the provider file.
			pf := m.providerForm.ToProviderFile()
			_, err := saveProviderFile(pf, m.cfg.ModelsDir)
			if err == nil {
				if m.gwLifecycle.IsRunning() {
					m.restartRequired = true
				}
			}
			// Reload the provider list.
			providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
			m.providerListItems = providers
			m.singleModelItems = singles
			m.providersState = pList
		}
		if m.providerForm.Cancelled() {
			m.providersState = pList
		}
		return m, cmd

	// -----------------------------------------------------------------------
	// Model form states (add / edit)
	// -----------------------------------------------------------------------
	case pAddModel, pEditModel:
		var cmd tea.Cmd
		m.modelForm, cmd = m.modelForm.Update(msg)
		if m.modelForm.Finished() {
			// Save the model config file.
			mf := m.modelForm.ToModelConfigFile()
			_, err := saveModelConfigFile(mf, m.cfg.ModelsDir)
			if err == nil {
				if m.gwLifecycle.IsRunning() {
					m.restartRequired = true
				}
			}
			// Reload the list.
			providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
			m.providerListItems = providers
			m.singleModelItems = singles
			m.providersState = pList
		}
		if m.modelForm.Cancelled() {
			m.providersState = pList
		}
		return m, cmd
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// clampWidth returns width clamped to max, with a minimum of 20.
func clampWidth(width, max int) int {
	if width > max {
		return max
	}
	if width < 20 {
		return 20
	}
	return width
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
