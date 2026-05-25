package forms

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"wolink-core/internal/models"
)

// Model step constants.
const (
	modelStepID     = 0
	modelStepName   = 1
	modelStepModel  = 2
	modelStepType   = 3
	modelStepMode   = 4
	modelStepRoute  = 5
	totalModelSteps = 6
)

// ModelFormModel is a form for adding or editing a single model (non-provider format).
// It presents a series of fields: ID, name, upstream model name, type, mode, route.
type ModelFormModel struct {
	id    textinput.Model
	name  textinput.Model
	model textinput.Model
	mtype textinput.Model
	mode  textinput.Model
	route textinput.Model

	step        int
	errors      []string
	width       int
	finished    bool
	cancelled   bool
	editing     bool
	editingID   string
}

// NewModelForm returns an initialized ModelFormModel.
func NewModelForm() ModelFormModel {
	return ModelFormModel{
		id:    newTextInput("Model ID: ", 40),
		name:  newTextInput("Display Name: ", 40),
		model: newTextInput("Upstream Model Name: ", 40),
		mtype: newTextInput("Type (chat/embedding/rerank/audio/ocr): ", 40),
		mode:  newTextInput("Mode (passthrough/parsed): ", 40),
		route: newTextInput("Route (random/balance/fastest): ", 40),
		step:  0,
	}
}

// NewModelFormEdit creates a form pre-filled from an existing ModelDef.
func NewModelFormEdit(def *models.ModelDef) ModelFormModel {
	f := NewModelForm()
	f.id.SetValue(def.ID)
	f.name.SetValue(def.Name)
	f.model.SetValue(def.Model)
	f.mtype.SetValue(def.Type)
	f.mode.SetValue(def.Mode)
	f.route.SetValue(def.Route)
	f.editing = true
	f.editingID = def.ID
	return f
}

// Init returns a tea.Cmd that focuses the first input field.
func (m ModelFormModel) Init() tea.Cmd {
	return m.activeInput().Focus()
}

// activeInput returns a pointer to the textinput for the current step.
func (m *ModelFormModel) activeInput() *textinput.Model {
	switch m.step {
	case modelStepID:
		return &m.id
	case modelStepName:
		return &m.name
	case modelStepModel:
		return &m.model
	case modelStepType:
		return &m.mtype
	case modelStepMode:
		return &m.mode
	case modelStepRoute:
		return &m.route
	default:
		return &m.id
	}
}

// Update handles messages and keyboard input for the model form.
func (m ModelFormModel) Update(msg tea.Msg) (ModelFormModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if m.step == totalModelSteps-1 {
				// Last step - validate and finish.
				if ok, errMsg := m.validateStep(); ok {
					m.finished = true
				} else {
					m.errors = []string{errMsg}
				}
				return m, nil
			}
			if ok, errMsg := m.validateStep(); ok {
				m.step++
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
		m.errors = nil
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	}

	return m, nil
}

// validateStep validates the current form step.
func (m *ModelFormModel) validateStep() (bool, string) {
	switch m.step {
	case modelStepID:
		return ValidateModelID(m.id.Value())
	case modelStepName:
		return ValidateRequired(m.name.Value())
	case modelStepModel:
		return ValidateRequired(m.model.Value())
	case modelStepType:
		return ValidateRequired(m.mtype.Value())
	case modelStepMode:
		return ValidateRequired(m.mode.Value())
	case modelStepRoute:
		return ValidateRequired(m.route.Value())
	default:
		return true, ""
	}
}

// View renders the current form step.
func (m ModelFormModel) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(" Step %d/%d\n", m.step+1, totalModelSteps))
	b.WriteString(strings.Repeat("─", min(m.width, 60)))
	b.WriteString("\n\n")

	switch m.step {
	case modelStepID:
		b.WriteString(" Enter the model ID (no spaces):\n")
		b.WriteString(" └─ ")
		b.WriteString(m.id.View())
		b.WriteByte('\n')

	case modelStepName:
		b.WriteString(" Enter the display name:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.name.View())
		b.WriteByte('\n')

	case modelStepModel:
		b.WriteString(" Enter the upstream model name:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.model.View())
		b.WriteByte('\n')

	case modelStepType:
		b.WriteString(" Enter the model type:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.mtype.View())
		b.WriteString("\n   Accepted: chat, embedding, rerank, audio, ocr\n")

	case modelStepMode:
		b.WriteString(" Enter the mode:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.mode.View())
		b.WriteString("\n   Accepted: passthrough, parsed\n")

	case modelStepRoute:
		b.WriteString(" Enter the routing strategy:\n")
		b.WriteString(" └─ ")
		b.WriteString(m.route.View())
		b.WriteString("\n   Accepted: random, balance, fastest\n")
	}

	for _, err := range m.errors {
		b.WriteString("\n ")
		b.WriteString(err)
		b.WriteByte('\n')
	}

	return b.String()
}

// ToModelDef produces a ModelDef with the current form values.
// Optional fields (Probe, Defaults, Capability) are left as zero values.
func (m ModelFormModel) ToModelDef() *models.ModelDef {
	id := m.id.Value()
	if m.editing && id == "" {
		id = m.editingID
	}

	return &models.ModelDef{
		ID:    id,
		Name:  m.name.Value(),
		Model: m.model.Value(),
		Type:  m.mtype.Value(),
		Mode:  m.mode.Value(),
		Route: m.route.Value(),
	}
}

// ToModelConfigFile produces a single-model config file from the form values.
func (m ModelFormModel) ToModelConfigFile() *models.ModelConfigFile {
	id := m.id.Value()
	if m.editing && id == "" {
		id = m.editingID
	}

	return &models.ModelConfigFile{
		ID:    id,
		Name:  m.name.Value(),
		Type:  m.mtype.Value(),
		Mode:  m.mode.Value(),
		Route: m.route.Value(),
		Meta: models.MetaConfig{
			Protocol: m.mtype.Value(),
		},
		ConnConfig: models.ConnectionConfig{
			Model: m.model.Value(),
		},
	}
}

// Finished returns true when the form has been submitted.
func (m ModelFormModel) Finished() bool {
	return m.finished
}

// Cancelled returns true when the user pressed Esc.
func (m ModelFormModel) Cancelled() bool {
	return m.cancelled
}

// IsEditing returns true if this form is editing an existing model.
func (m ModelFormModel) IsEditing() bool {
	return m.editing
}
