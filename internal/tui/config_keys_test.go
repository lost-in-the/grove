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

// Only the active section's form gets messages, so its cursor blink stops
// while another section shows. Arriving at a section refocuses its field,
// which starts the blink again, by every way of changing section.
func TestConfig_SectionChangeRefocuses(t *testing.T) {
	cases := []struct {
		name string
		from ConfigTab
		keys []string
		to   ConfigTab
	}{
		{"ctrl+p", ConfigTabBehavior, []string{"ctrl+p"}, ConfigTabGeneral},
		{"ctrl+n", ConfigTabPlugins, []string{"ctrl+n"}, ConfigTabProtection},
		{"digit", ConfigTabPlugins, []string{"1"}, ConfigTabGeneral},
		{"finishing plugins", ConfigTabPlugins, []string{"enter", "enter", "enter"}, ConfigTabProtection},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel() // each case waits out a blink interval
			m := openConfigWith(t, 100, 30, nil)
			m.configState.Tab = c.from
			last := len(c.keys) - 1
			m = sendConfigKeys(t, m, c.keys[:last]...)
			m, slow := sendConfigKey(t, m, c.keys[last])
			if m.configState.Tab != c.to {
				t.Fatalf("section %s, want %s", tabName(m.configState.Tab), tabName(c.to))
			}
			if !startsBlink(slow) {
				t.Errorf("arriving at %s should start its input's cursor blink", tabName(c.to))
			}
		})
	}
}

// enter on a last field finishes the section through huh's follow-up
// messages. A section key landing before or between them must not pass
// them to the new section's form, where they would move its focus or
// finish it.
func TestConfig_SectionKeyDuringFinish(t *testing.T) {
	for _, steps := range []int{0, 1} { // follow-ups handled before ctrl+n
		m := openConfigWith(t, 100, 30, nil)
		m = sendConfigKeys(t, m, "tab", "tab") // general's last field
		next, cmd := m.Update(makeKeyMsg("enter"))
		m = next.(Model)
		pending := runCmd(cmd)
		for range steps {
			next, cmd = m.Update(pending[0])
			m = next.(Model)
			pending = append(pending[1:], runCmd(cmd)...)
		}
		m = sendConfigKeys(t, m, "ctrl+n")
		for _, msg := range pending {
			m, _ = sendConfigMsg(t, m, msg)
		}
		if m.configState.Tab != ConfigTabBehavior || focusedKey(m) != "switch.dirty_handling" {
			t.Errorf("after %d follow-ups: section %s, focus %q; want Behavior on its first field",
				steps, tabName(m.configState.Tab), focusedKey(m))
		}
		if m = sendConfigKeys(t, m, "tab"); focusedKey(m) != "tmux.mode" {
			t.Errorf("after %d follow-ups, tab in Behavior: focus %q, want tmux.mode", steps, focusedKey(m))
		}
	}
}
