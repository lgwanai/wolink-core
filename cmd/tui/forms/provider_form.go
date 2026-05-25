package forms

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"wolink-core/internal/models"
)

// Wizard step constants.
const (
	providerStepName        = 0
	providerStepProtocol    = 1
	providerStepBaseURL     = 2
	providerStepAPIKey      = 3
	providerStepDescription = 4
	providerStepConfirm     = 5
	totalProviderSteps      = 6
)

// ProviderFormModel is a guided wizard for adding or editing a provider.
// The wizard progresses through steps: name -> protocol -> base URL ->
// API key -> description -> confirm. Each step validates before advancing.
type ProviderFormModel struct {
	name        textinput.Model
	protocol    textinput.Model
	baseURL     textinput.Model
	apiKey      textinput.Model
	description textinput.Model

	step int

	collectedModels []models.ModelDef
	errors          []string
	width           int
	finished        bool
	cancelled       bool
	existingID      string
}

// NewProviderForm returns an initialized ProviderFormModel for creating a new provider.
func NewProviderForm() ProviderFormModel {
	f := ProviderFormModel{
		name:        newTextInput("Provider Name: ", 50),
		protocol:    newTextInput("Protocol (openai/deepseek/claude/custom): ", 50),
		baseURL:     newTextInput("Base URL (http://...): ", 60),
		apiKey:      newPasswordInput("API Key: ", 60),
		description: newTextInput("Description (optional): ", 60),
		step:        0,
	}
	return f
}

// NewProviderFormEdit creates a form pre-filled from an existing ProviderFile.
func NewProviderFormEdit(pf *models.ProviderFile) ProviderFormModel {
	f := NewProviderForm()
	f.name.SetValue(pf.Name)
	f.protocol.SetValue(pf.Protocol)
	f.baseURL.SetValue(pf.BaseURL)
	f.apiKey.SetValue(pf.APIKey)
	if desc, ok := pf.Description["en"]; ok {
		f.description.SetValue(desc)
	} else if desc, ok := pf.Description["zh"]; ok {
		f.description.SetValue(desc)
	}
	f.collectedModels = pf.Models
	f.existingID = pf.ID
	return f
}

// newTextInput creates a textinput.Model with prompt and width set.
func newTextInput(prompt string, width int) textinput.Model {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.SetWidth(width)
	return ti
}

// newPasswordInput creates a textinput.Model with echo password mode.
func newPasswordInput(prompt string, width int) textinput.Model {
	ti := textinput.New()
	ti.Prompt = prompt
	ti.SetWidth(width)
	ti.EchoMode = textinput.EchoPassword
	return ti
}

// Init returns a tea.Cmd that focuses the first input field.
func (m ProviderFormModel) Init() tea.Cmd {
	return m.activeInput().Focus()
}

// activeInput returns a pointer to the textinput for the current step.
func (m *ProviderFormModel) activeInput() *textinput.Model {
	switch m.step {
	case providerStepName:
		return &m.name
	case providerStepProtocol:
		return &m.protocol
	case providerStepBaseURL:
		return &m.baseURL
	case providerStepAPIKey:
		return &m.apiKey
	case providerStepDescription:
		return &m.description
	default:
		return &m.name
	}
}

// Update handles messages and keyboard input for the provider form.
func (m ProviderFormModel) Update(msg tea.Msg) (ProviderFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if m.step == providerStepConfirm {
				m.finished = true
				return m, nil
			}
			if ok, errMsg := m.validateStep(); ok {
				// Advance to next step and focus the new input.
				m.step++
				// Clear errors on successful advance.
				m.errors = nil
				cmd := m.activeInput().Focus()
				return m, cmd
			} else {
				m.errors = []string{errMsg}
				return m, nil
			}

		case "esc":
			m.cancelled = true
			return m, nil
		}

		// Delegate to the active textinput for text entry.
		old := *m.activeInput()
		updated, cmd := old.Update(msg)
		*m.activeInput() = updated
		// Clear step-level errors when user types.
		m.errors = nil
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	}

	return m, nil
}

// validateStep validates the current wizard step and returns an error message
// if the value is invalid.
func (m *ProviderFormModel) validateStep() (bool, string) {
	switch m.step {
	case providerStepName:
		return ValidateRequired(m.name.Value())
	case providerStepProtocol:
		return ValidateProtocol(m.protocol.Value())
	case providerStepBaseURL:
		return ValidateBaseURL(m.baseURL.Value())
	case providerStepAPIKey:
		return ValidateRequired(m.apiKey.Value())
	case providerStepDescription:
		return true, "" // optional
	default:
		return true, ""
	}
}

// View renders the current wizard step.
func (m ProviderFormModel) View() string {
	var b strings.Builder

	// Progress indicator
	b.WriteString(fmt.Sprintf(" Step %d/%d\n", m.step+1, totalProviderSteps))
	b.WriteString(strings.Repeat("─", min(m.width, 60)))
	b.WriteString("\n\n")

	// Render the current step.
	switch m.step {
	case providerStepName:
		b.WriteString(" Enter a name for this provider:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.name.View())
		b.WriteByte('\n')

	case providerStepProtocol:
		b.WriteString(" Select the API protocol:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.protocol.View())
		b.WriteString("\n   Accepted: openai, deepseek, claude, custom\n")

	case providerStepBaseURL:
		b.WriteString(" Enter the provider's base URL:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.baseURL.View())
		b.WriteByte('\n')

	case providerStepAPIKey:
		b.WriteString(" Enter the API key for this provider:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.apiKey.View())
		b.WriteByte('\n')

	case providerStepDescription:
		b.WriteString(" Enter an optional description:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.description.View())
		b.WriteByte('\n')

	case providerStepConfirm:
		b.WriteString(" Review and confirm:\n\n")
		b.WriteString(fmt.Sprintf("   Name:        %s\n", m.name.Value()))
		b.WriteString(fmt.Sprintf("   Protocol:    %s\n", m.protocol.Value()))
		b.WriteString(fmt.Sprintf("   Base URL:    %s\n", m.baseURL.Value()))
		b.WriteString(fmt.Sprintf("   API Key:     %s\n", maskString(m.apiKey.Value(), 8)))
		desc := m.description.Value()
		if desc == "" {
			desc = "(none)"
		}
		b.WriteString(fmt.Sprintf("   Description: %s\n", desc))
		b.WriteString(fmt.Sprintf("   Models:      %d configured\n\n", len(m.collectedModels)))
		b.WriteString(" Press Enter to save, Esc to cancel\n")
	}

	// Render validation errors.
	for _, err := range m.errors {
		b.WriteString("\n ")
		b.WriteString(err)
		b.WriteByte('\n')
	}

	return b.String()
}

// ToProviderFile assembles a ProviderFile from the current form values.
func (m ProviderFormModel) ToProviderFile() *models.ProviderFile {
	id := m.existingID
	if id == "" {
		id = sanitizeID(m.name.Value())
	}

	desc := make(map[string]string)
	if d := m.description.Value(); d != "" {
		desc["en"] = d
	}

	return &models.ProviderFile{
		ID:          id,
		Name:        m.name.Value(),
		Protocol:    m.protocol.Value(),
		BaseURL:     m.baseURL.Value(),
		APIKey:      m.apiKey.Value(),
		Description: desc,
		Models:      m.collectedModels,
	}
}

// SetModels replaces the collected models slice.
// Used when editing an existing provider's model list.
func (m *ProviderFormModel) SetModels(models []models.ModelDef) {
	m.collectedModels = models
}

// Value returns the core form values (name, protocol, baseURL, apiKey).
func (m ProviderFormModel) Value() (string, string, string, string) {
	return m.name.Value(), m.protocol.Value(), m.baseURL.Value(), m.apiKey.Value()
}

// Finished returns true when the wizard has been completed.
func (m ProviderFormModel) Finished() bool {
	return m.finished
}

// Cancelled returns true when the user pressed Esc during the wizard.
func (m ProviderFormModel) Cancelled() bool {
	return m.cancelled
}

// ExistingID returns the ID of the provider being edited, or "" for new.
func (m ProviderFormModel) ExistingID() string {
	return m.existingID
}

// CollectedModels returns the current list of models.
func (m ProviderFormModel) CollectedModels() []models.ModelDef {
	return m.collectedModels
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// maskString returns the string with all but the first n characters replaced by '*'.
func maskString(s string, visible int) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= visible {
		return s
	}
	return s[:visible] + strings.Repeat("*", len(s)-visible)
}

// sanitizeID converts a display name into a usable YAML key.
func sanitizeID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	return id
}
