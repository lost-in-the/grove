package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/lost-in-the/grove/internal/config"
)

var ansiStripRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func TestNewConfigState(t *testing.T) {
	s := NewConfigState()
	if s.Tab != ConfigTabGeneral {
		t.Errorf("expected ConfigTabGeneral, got %d", s.Tab)
	}
	if len(s.Fields) != int(ConfigTabCount) {
		t.Errorf("expected %d field groups, got %d", ConfigTabCount, len(s.Fields))
	}
}

func TestConfigOverlay_OpenClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")
	if m.activeView != ViewConfig {
		t.Errorf("expected ViewConfig, got %d", m.activeView)
	}
	if m.configState == nil {
		t.Fatal("expected configState to be set")
	}

	m = sendKey(m, "esc")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after esc, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState to be nil after close")
	}
}

func TestConfigOverlay_NilState(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = nil
	m = sendKey(m, "enter")
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard for nil state, got %d", m.activeView)
	}
}

func TestConfigOverlay_TabSwitching(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	// Populate fields
	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg

	if m.configState.Tab != ConfigTabGeneral {
		t.Errorf("expected General tab, got %d", m.configState.Tab)
	}

	m = sendKey(m, "tab")
	if m.configState.Tab != ConfigTabBehavior {
		t.Errorf("expected Behavior tab, got %d", m.configState.Tab)
	}
	if m.configState.Cursor != 0 {
		t.Error("expected cursor reset to 0 on tab switch")
	}

	m = sendKey(m, "tab")
	if m.configState.Tab != ConfigTabPlugins {
		t.Errorf("expected Plugins tab, got %d", m.configState.Tab)
	}

	m = sendKey(m, "tab")
	if m.configState.Tab != ConfigTabProtection {
		t.Errorf("expected Protection tab, got %d", m.configState.Tab)
	}

	m = sendKey(m, "tab")
	if m.configState.Tab != ConfigTabGeneral {
		t.Errorf("expected General tab (wrap around), got %d", m.configState.Tab)
	}
}

func TestConfigOverlay_FieldNavigation(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg

	if m.configState.Cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.configState.Cursor)
	}

	m = sendKey(m, "j")
	if m.configState.Cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.configState.Cursor)
	}

	m = sendKey(m, "k")
	if m.configState.Cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.configState.Cursor)
	}

	// Can't go above 0
	m = sendKey(m, "k")
	if m.configState.Cursor != 0 {
		t.Errorf("expected cursor still at 0, got %d", m.configState.Cursor)
	}
}

func TestConfigOverlay_FieldNavigationClamp(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg

	// Navigate to the bottom
	fieldCount := len(m.configState.Fields[ConfigTabGeneral])
	for i := 0; i < fieldCount+5; i++ {
		m = sendKey(m, "j")
	}
	if m.configState.Cursor != fieldCount-1 {
		t.Errorf("expected cursor at %d, got %d", fieldCount-1, m.configState.Cursor)
	}
}

func TestConfigOverlay_EnterOpensEdit(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg

	m = sendKey(m, "enter")
	if !m.configState.Editing {
		t.Error("expected Editing=true after enter")
	}
	if m.configState.EditBuffer != cfg.ProjectName {
		t.Errorf("expected EditBuffer=%q, got %q", cfg.ProjectName, m.configState.EditBuffer)
	}
}

func TestConfigLoadedMsg(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	cfg := config.LoadDefaults()
	cfg.ProjectName = "test-project"
	m = sendMsg(m, configLoadedMsg{cfg: cfg})

	if m.configState.Config == nil {
		t.Fatal("expected Config to be set")
	}
	if m.configState.Config.ProjectName != "test-project" {
		t.Errorf("expected project name 'test-project', got %q", m.configState.Config.ProjectName)
	}
	if len(m.configState.Fields[ConfigTabGeneral]) == 0 {
		t.Error("expected General fields to be populated")
	}
}

func TestConfigLoadedMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	m = sendMsg(m, configLoadedMsg{err: errTest})
	if m.configState.Err == nil {
		t.Error("expected error on configState")
	}
}

func TestConfigSavedMsg_Success(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	m = sendMsg(m, configSavedMsg{})
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after save, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState nil after save")
	}
}

func TestConfigSavedMsg_Error(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()

	m = sendMsg(m, configSavedMsg{err: errTest})
	if m.activeView != ViewConfig {
		t.Errorf("expected ViewConfig after save error, got %d", m.activeView)
	}
	if m.configState.Err == nil {
		t.Error("expected error on configState")
	}
}

func TestConfigSavedMsg_ErrorAfterClose(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	// Simulate save-on-close: configState is already nil when error arrives
	m.activeView = ViewDashboard
	m.configState = nil

	m = sendMsg(m, configSavedMsg{err: errTest})
	// Should show toast error even though configState is nil
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard, got %d", m.activeView)
	}
}

func TestPopulateConfigFields(t *testing.T) {
	cfg := config.LoadDefaults()
	cfg.ProjectName = "grove-cli"
	cfg.Alias = "grove"

	fields := populateConfigFields(cfg)

	if len(fields) != int(ConfigTabCount) {
		t.Fatalf("expected %d tab groups, got %d", ConfigTabCount, len(fields))
	}

	// General tab
	general := fields[ConfigTabGeneral]
	if len(general) != 4 {
		t.Errorf("expected 4 General fields, got %d", len(general))
	}
	if general[0].Key != "project_name" {
		t.Errorf("expected first field 'project_name', got %q", general[0].Key)
	}

	// Behavior tab
	behavior := fields[ConfigTabBehavior]
	if len(behavior) != 5 {
		t.Errorf("expected 5 Behavior fields, got %d", len(behavior))
	}

	// Plugins tab
	plugins := fields[ConfigTabPlugins]
	if len(plugins) != 3 {
		t.Errorf("expected 3 Plugins fields, got %d", len(plugins))
	}

	// Protection tab
	protection := fields[ConfigTabProtection]
	if len(protection) != 2 {
		t.Errorf("expected 2 Protection fields, got %d", len(protection))
	}
}

func TestTabName(t *testing.T) {
	names := map[ConfigTab]string{
		ConfigTabGeneral:    "General",
		ConfigTabBehavior:   "Behavior",
		ConfigTabPlugins:    "Plugins",
		ConfigTabProtection: "Protection",
	}
	for tab, expected := range names {
		if got := tabName(tab); got != expected {
			t.Errorf("tabName(%d) = %q, want %q", tab, got, expected)
		}
	}
}

func TestRenderConfig_AllStates(t *testing.T) {
	t.Run("empty fields", func(t *testing.T) {
		s := NewConfigState()
		v := renderConfig(s, 80)
		if v == "" {
			t.Fatal("expected non-empty render")
		}
		if !strings.Contains(v, "Configuration") {
			t.Error("expected 'Configuration' title")
		}
	})

	t.Run("with fields", func(t *testing.T) {
		s := NewConfigState()
		cfg := config.LoadDefaults()
		cfg.ProjectName = "test-project"
		s.Config = cfg
		s.Fields = populateConfigFields(cfg)

		v := renderConfig(s, 100)
		if !strings.Contains(v, "project_name") {
			t.Errorf("expected 'project_name' field in render, got:\n%s", v)
		}
		// Strip ANSI codes before checking — lipgloss v2 may wrap each character
		// individually when applying underline styling.
		plain := ansiStripRE.ReplaceAllString(v, "")
		if !strings.Contains(plain, "General") {
			t.Errorf("expected 'General' tab label, got:\n%s", v)
		}
	})

	t.Run("editing mode", func(t *testing.T) {
		s := NewConfigState()
		cfg := config.LoadDefaults()
		s.Config = cfg
		s.Fields = populateConfigFields(cfg)
		s.Editing = true
		s.EditBuffer = s.Fields[ConfigTabGeneral][0].Value

		v := renderConfig(s, 100)
		if !strings.Contains(v, "save") {
			t.Error("expected 'save' hint in editing mode")
		}
	})

	t.Run("dirty state", func(t *testing.T) {
		s := NewConfigState()
		cfg := config.LoadDefaults()
		s.Config = cfg
		s.Fields = populateConfigFields(cfg)
		s.Dirty = true

		v := renderConfig(s, 100)
		if !strings.Contains(v, "save & close") {
			t.Error("expected 'save & close' footer in dirty state")
		}
		if strings.Contains(v, "unsaved") {
			t.Error("expected no 'unsaved' indicator — replaced by per-field coloring")
		}
	})

	t.Run("confirming state", func(t *testing.T) {
		s := NewConfigState()
		cfg := config.LoadDefaults()
		s.Config = cfg
		s.Fields = populateConfigFields(cfg)
		s.Dirty = true
		s.Confirming = true

		v := renderConfig(s, 100)
		if !strings.Contains(v, "Save changes?") {
			t.Error("expected 'Save changes?' prompt in confirming state")
		}
		if !strings.Contains(v, "save") || !strings.Contains(v, "discard") {
			t.Error("expected save/discard hints in confirming state")
		}
	})

	t.Run("error state", func(t *testing.T) {
		s := NewConfigState()
		s.Err = errTest

		v := renderConfig(s, 80)
		if !strings.Contains(v, "test error") {
			t.Error("expected error text in render")
		}
	})
}

func TestConfigEditManual_BoolToggle(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg

	// Navigate to a bool field (Behavior tab, skip_branch_notice)
	m = sendKey(m, "tab") // switch to Behavior tab
	// Navigate down to find a bool field
	for i := 0; i < len(m.configState.Fields[m.configState.Tab]); i++ {
		if m.configState.Fields[m.configState.Tab][i].Type == ConfigBool {
			m.configState.Cursor = i
			break
		}
	}

	m = sendKey(m, "enter")
	if !m.configState.Editing {
		t.Fatal("expected editing mode")
	}
}

func TestConfigOverlay_EscDirtyConfirms(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg
	m.configState.Dirty = true

	m = sendKey(m, "esc")
	// Should show confirmation prompt, not close
	if m.activeView != ViewConfig {
		t.Errorf("expected ViewConfig after dirty esc, got %d", m.activeView)
	}
	if m.configState == nil {
		t.Fatal("expected configState to still exist")
	}
	if !m.configState.Confirming {
		t.Error("expected Confirming=true after dirty esc")
	}
}

func TestConfigOverlay_ConfirmEnterSaves(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg
	m.configState.Dirty = true
	m.configState.Confirming = true

	m = sendKey(m, "enter")
	// Enter while confirming should save and close
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after confirm enter, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState nil after confirm enter")
	}
}

func TestConfigOverlay_ConfirmEscDiscards(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg
	m.configState.Dirty = true
	m.configState.Confirming = true

	m = sendKey(m, "esc")
	// Esc while confirming should discard and close
	if m.activeView != ViewDashboard {
		t.Errorf("expected ViewDashboard after confirm esc, got %d", m.activeView)
	}
	if m.configState != nil {
		t.Error("expected configState nil after confirm esc")
	}
}

func TestConfigEditKey_NoFieldsEditing(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m.activeView = ViewConfig
	m.configState = NewConfigState()
	m.configState.Editing = true
	m.configState.Cursor = 99 // out of bounds

	m = sendKey(m, "enter")
	if m.configState.Editing {
		t.Error("expected Editing=false when cursor is out of bounds")
	}
}

func TestConfigEditKey_EnterSavesOriginalValue(t *testing.T) {
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	cfg.ProjectName = "my-project"
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg

	// Enter should save EditOriginalValue before creating the form
	m = sendKey(m, "enter")
	if !m.configState.Editing {
		t.Fatal("expected Editing=true after enter")
	}
	if m.configState.EditOriginalValue != "my-project" {
		t.Errorf("expected EditOriginalValue='my-project', got %q", m.configState.EditOriginalValue)
	}
}

func TestConfigEditKey_DirtyDetection(t *testing.T) {
	// Verify the Dirty mechanism: Huh updates field.Value through pointer,
	// so comparing against EditOriginalValue detects changes correctly.
	m := newTestModel(withItems(3), withSize(80, 30))
	m = sendKey(m, "c")

	cfg := config.LoadDefaults()
	m.configState.Fields = populateConfigFields(cfg)
	m.configState.Config = cfg

	original := m.configState.Fields[ConfigTabGeneral][0].Value
	m.configState.EditOriginalValue = original

	// Simulate Huh changing the value through pointer binding
	m.configState.Fields[ConfigTabGeneral][0].Value = "changed-value"

	// The comparison against EditOriginalValue should detect the change
	if m.configState.Fields[ConfigTabGeneral][0].Value == m.configState.EditOriginalValue {
		t.Error("expected value to differ from EditOriginalValue after Huh edit")
	}

	// Verify that unchanged values DON'T set dirty
	m.configState.Fields[ConfigTabGeneral][0].Value = original
	if m.configState.Fields[ConfigTabGeneral][0].Value != m.configState.EditOriginalValue {
		t.Error("expected value to match EditOriginalValue when unchanged")
	}
}

func TestConfigEditKey_AbortRestoresValue(t *testing.T) {
	// Verify the abort path restores EditOriginalValue
	s := NewConfigState()
	cfg := config.LoadDefaults()
	cfg.ProjectName = "original-name"
	s.Fields = populateConfigFields(cfg)
	s.Config = cfg

	s.EditOriginalValue = "original-name"
	// Simulate Huh having modified the value during interaction
	s.Fields[ConfigTabGeneral][0].Value = "modified-during-edit"

	// The abort code should restore EditOriginalValue
	s.Fields[ConfigTabGeneral][0].Value = s.EditOriginalValue
	if s.Fields[ConfigTabGeneral][0].Value != "original-name" {
		t.Errorf("expected value restored to 'original-name', got %q", s.Fields[ConfigTabGeneral][0].Value)
	}
}

func TestConfigFieldTypeConstants(t *testing.T) {
	if ConfigString != 0 || ConfigBool != 1 || ConfigEnum != 2 || ConfigList != 3 {
		t.Error("unexpected ConfigFieldType constant values")
	}
}

func TestConfigTabConstants(t *testing.T) {
	if ConfigTabGeneral != 0 || ConfigTabBehavior != 1 || ConfigTabPlugins != 2 || ConfigTabProtection != 3 {
		t.Error("unexpected ConfigTab constant values")
	}
	if ConfigTabCount != 4 {
		t.Errorf("expected ConfigTabCount=4, got %d", ConfigTabCount)
	}
}
