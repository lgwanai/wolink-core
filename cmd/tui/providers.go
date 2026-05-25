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

// renderProvidersList renders the provider list view with action items.
func renderProvidersList(m model) string {
	var b strings.Builder

	heading := m.styles.titleStyle.Render("Providers")
	b.WriteString(fmt.Sprintf(" %s\n", heading))
	b.WriteString(" " + strings.Repeat("─", clampWidth(m.width-2, 60)) + "\n\n")

	// Action items at top
	b.WriteString(renderProvidersActions(m))
	b.WriteString("\n")

	if len(m.providerListItems) == 0 && len(m.singleModelItems) == 0 {
		b.WriteString("   No providers or models configured.\n")
		return b.String()
	}

	if len(m.providerListItems) > 0 {
		b.WriteString("   Providers:\n")
		for i, item := range m.providerListItems {
			cursorIdx := 3 + i // action buttons are 0,1,2
			cursor := "  "
			if m.contentCursor == cursorIdx {
				cursor = m.styles.actionActive.Render(" >")
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
			cursorIdx := 3 + len(m.providerListItems) + i
			cursor := "  "
			if m.contentCursor == cursorIdx {
				cursor = m.styles.actionActive.Render(" >")
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

// renderProvidersActions renders the visible action buttons for the providers tab.
func renderProvidersActions(m model) string {
	actions := []string{"[Add Provider]", "[Add Single Model]", "[Delete Selected]"}
	var rendered []string
	for i, label := range actions {
		if m.contentCursor == i {
			rendered = append(rendered, m.styles.actionActive.Render(label))
		} else {
			rendered = append(rendered, m.styles.actionItem.Render(label))
		}
	}
	return "  " + strings.Join(rendered, "  ") + "\n"
}

// ---------------------------------------------------------------------------
// Content enter handling
// ---------------------------------------------------------------------------

func handleProvidersContentEnter(m model) (tea.Model, tea.Cmd) {
	c := m.contentCursor

	// Action buttons (0, 1, 2)
	if c == 0 {
		m.providersState = pAddProvider
		m.providerForm = forms.NewProviderForm()
		return m, m.providerForm.Init()
	}
	if c == 1 {
		m.providersState = pAddModel
		m.modelForm = forms.NewModelForm()
		return m, m.modelForm.Init()
	}
	if c == 2 {
		if len(m.providerListItems) == 0 && len(m.singleModelItems) == 0 {
			return m, nil
		}
		// Delete selected provider (if cursor is on one)
		provIdx := c - 3
		if provIdx >= 0 && provIdx < len(m.providerListItems) {
			item := m.providerListItems[provIdx]
			if err := deleteProviderFile(item.filePath); err == nil {
				providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
				m.providerListItems = providers
				m.singleModelItems = singles
				if m.gwLifecycle.IsRunning() {
					m.restartRequired = true
				}
			}
		}
		return m, nil
	}

	// Provider list items (index 3+)
	provIdx := c - 3
	if provIdx >= 0 && provIdx < len(m.providerListItems) {
		item := m.providerListItems[provIdx]
		m.providersState = pEditProvider
		m.providerForm = forms.NewProviderFormEdit(item.provider)
		return m, m.providerForm.Init()
	}

	// Single model items
	singleIdx := c - 3 - len(m.providerListItems)
	if singleIdx >= 0 && singleIdx < len(m.singleModelItems) {
		item := m.singleModelItems[singleIdx]
		m.providersState = pEditModel
		def := &models.ModelDef{
			ID:    item.model.ID,
			Name:  item.model.Name,
			Model: item.model.ConnConfig.Model,
			Type:  item.model.Type,
			Mode:  item.model.Mode,
			Route: item.model.Route,
		}
		m.modelForm = forms.NewModelFormEdit(def)
		return m, m.modelForm.Init()
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Key handling (forms only — list navigation is handled by model.go)
// ---------------------------------------------------------------------------

// handleProvidersKeyMsg handles key events for provider/model forms.
func handleProvidersKeyMsg(m model, msg tea.KeyMsg) (model, tea.Cmd) {
	switch m.providersState {
	case pAddProvider, pEditProvider:
		var cmd tea.Cmd
		m.providerForm, cmd = m.providerForm.Update(msg)
		if m.providerForm.Finished() {
			pf := m.providerForm.ToProviderFile()
			_, err := saveProviderFile(pf, m.cfg.ModelsDir)
			if err == nil {
				if m.gwLifecycle.IsRunning() {
					m.restartRequired = true
				}
			}
			providers, singles, _ := listProviderFiles(m.cfg.ModelsDir)
			m.providerListItems = providers
			m.singleModelItems = singles
			m.providersState = pList
			}
		if m.providerForm.Cancelled() {
			m.providersState = pList
		}
		return m, cmd

	case pAddModel, pEditModel:
		var cmd tea.Cmd
		m.modelForm, cmd = m.modelForm.Update(msg)
		if m.modelForm.Finished() {
			mf := m.modelForm.ToModelConfigFile()
			_, err := saveModelConfigFile(mf, m.cfg.ModelsDir)
			if err == nil {
				if m.gwLifecycle.IsRunning() {
					m.restartRequired = true
				}
			}
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
