package tui

import (
	"strings"
	"testing"

	"github.com/LeahArmstrong/grove-cli/internal/config"
)

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
	if m.configState.EditForm == nil {
		t.Error("expected EditForm to be set")
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
	if len(behavior) != 4 {
		t.Errorf("expected 4 Behavior fields, got %d", len(behavior))
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
			t.Error("expected 'project_name' field in render")
		}
		if !strings.Contains(v, "General") {
			t.Error("expected 'General' tab label")
		}
	})

	t.Run("editing mode", func(t *testing.T) {
		s := NewConfigState()
		cfg := config.LoadDefaults()
		s.Config = cfg
		s.Fields = populateConfigFields(cfg)
		s.Editing = true
		s.EditForm = newConfigEditForm(&s.Fields[ConfigTabGeneral][0])

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
		if !strings.Contains(v, "unsaved") {
			t.Error("expected 'unsaved' indicator in dirty state")
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

func TestNewConfigEditForm(t *testing.T) {
	t.Run("string field", func(t *testing.T) {
		field := &ConfigField{
			Key:   "test",
			Label: "Test",
			Value: "hello",
			Type:  ConfigString,
		}
		form := newConfigEditForm(field)
		if form == nil {
			t.Fatal("expected non-nil form")
		}
	})

	t.Run("bool field", func(t *testing.T) {
		field := &ConfigField{
			Key:   "test",
			Label: "Test",
			Value: "true",
			Type:  ConfigBool,
		}
		form := newConfigEditForm(field)
		if form == nil {
			t.Fatal("expected non-nil form")
		}
	})

	t.Run("enum field", func(t *testing.T) {
		field := &ConfigField{
			Key:     "test",
			Label:   "Test",
			Value:   "option1",
			Type:    ConfigEnum,
			Options: []string{"option1", "option2"},
		}
		form := newConfigEditForm(field)
		if form == nil {
			t.Fatal("expected non-nil form")
		}
	})

	t.Run("list field", func(t *testing.T) {
		field := &ConfigField{
			Key:   "test",
			Label: "Test",
			Value: "a, b, c",
			Type:  ConfigList,
		}
		form := newConfigEditForm(field)
		if form == nil {
			t.Fatal("expected non-nil form")
		}
	})
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
