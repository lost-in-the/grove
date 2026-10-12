package tui

import "testing"

func TestConfig_CtrlNPWrap(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m = sendConfigKeys(t, m, "ctrl+n")
	if m.configState.Tab != ConfigTabBehavior {
		t.Errorf("ctrl+n: section %s, want Behavior", tabName(m.configState.Tab))
	}
	m = sendConfigKeys(t, m, "ctrl+p", "ctrl+p")
	if m.configState.Tab != ConfigTabProtection {
		t.Errorf("ctrl+p twice from Behavior: section %s, want Protection (wraps)", tabName(m.configState.Tab))
	}
	m = sendConfigKeys(t, m, "ctrl+n")
	if m.configState.Tab != ConfigTabGeneral {
		t.Errorf("ctrl+n from Protection: section %s, want General (wraps)", tabName(m.configState.Tab))
	}
}

// Digits jump to a section only off text inputs, where they must type.
func TestConfig_DigitsJumpOutsideInputs(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m = sendConfigKeys(t, m, "3")
	if m.configState.Tab != ConfigTabGeneral || m.configState.Fields[ConfigTabGeneral][0].Value != "3" {
		t.Errorf("3 in an input: section %s, value %q; want it typed", tabName(m.configState.Tab), m.configState.Fields[ConfigTabGeneral][0].Value)
	}

	m = openConfigWith(t, 100, 30, nil)
	m.configState.Tab = ConfigTabPlugins // a confirm field has focus
	m = sendConfigKeys(t, m, "2")
	if m.configState.Tab != ConfigTabBehavior {
		t.Errorf("2 on a confirm: section %s, want Behavior", tabName(m.configState.Tab))
	}
	m = sendConfigKeys(t, m, "4") // a select has focus
	if m.configState.Tab != ConfigTabProtection {
		t.Errorf("4 on a select: section %s, want Protection", tabName(m.configState.Tab))
	}
}

func TestConfig_QuestionMarkTypesInInput(t *testing.T) {
	m := openConfigWith(t, 100, 30, nil)
	m = sendConfigKeys(t, m, "?")
	if m.activeView() != ViewConfig || m.configState.Fields[ConfigTabGeneral][0].Value != "?" {
		t.Errorf("? in an input: view %v, value %q; want it typed", m.activeView(), m.configState.Fields[ConfigTabGeneral][0].Value)
	}

	m = openConfigWith(t, 100, 30, nil)
	m.configState.Tab = ConfigTabPlugins
	if m = sendConfigKeys(t, m, "?"); m.activeView() != ViewHelp {
		t.Errorf("? off an input: view %v, want help", m.activeView())
	}
}
