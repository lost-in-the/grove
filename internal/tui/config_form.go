package tui

import (
	"charm.land/huh/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// groveHuhTheme returns a Huh theme styled to match grove's color palette.
func groveHuhTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		t := huh.ThemeBase(isDark)

		cs := Colors

		// Focused field styles
		t.Focused.Base = t.Focused.Base.BorderForeground(cs.Primary)
		t.Focused.Card = t.Focused.Base
		t.Focused.Title = t.Focused.Title.Foreground(cs.TextBright).Bold(true)
		t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(cs.Primary).Bold(true).MarginBottom(1)
		t.Focused.Description = t.Focused.Description.Foreground(cs.TextMuted)
		t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(cs.Danger)
		t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(cs.Danger)
		t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(cs.Info)
		t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(cs.Info)
		t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(cs.Info)
		t.Focused.Option = t.Focused.Option.Foreground(cs.TextNormal)
		t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(cs.Info)
		t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(cs.Success)
		t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(cs.Success).SetString("✓ ")
		t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(cs.TextMuted).SetString("• ")
		t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(cs.TextNormal)
		t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(cs.TextBright).Background(cs.Primary)
		t.Focused.Next = t.Focused.FocusedButton
		t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(cs.TextNormal).Background(cs.SurfaceBorder)

		t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(cs.Primary)
		t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(cs.TextDim)
		t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(cs.Info)

		// Blurred field styles — same as focused but with hidden border
		t.Blurred = t.Focused
		t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
		t.Blurred.Card = t.Blurred.Base
		t.Blurred.NextIndicator = lipgloss.NewStyle()
		t.Blurred.PrevIndicator = lipgloss.NewStyle()

		// Group styles
		t.Group.Title = t.Focused.Title
		t.Group.Description = t.Focused.Description

		return t
	})
}

// configFormValues holds the mutable value pointers that the Huh form binds to.
// This lets us extract values after form completion and detect dirty state.
type configFormValues struct {
	strings map[string]*string
	bools   map[string]*bool
}

// buildConfigForm creates a Huh form from the config fields, organized by tab.
// Returns the form and the value bindings so values can be read back.
func buildConfigForm(fields [][]ConfigField, width int) (*huh.Form, *configFormValues) {
	vals := &configFormValues{
		strings: make(map[string]*string),
		bools:   make(map[string]*bool),
	}

	var groups []*huh.Group

	for tab := ConfigTab(0); tab < ConfigTabCount; tab++ {
		tabFields := fields[tab]
		if len(tabFields) == 0 {
			continue
		}

		var huhFields []huh.Field
		for i := range tabFields {
			f := &tabFields[i]
			field := buildConfigHuhField(f, vals)
			if field != nil {
				huhFields = append(huhFields, field)
			}
		}

		if len(huhFields) > 0 {
			g := huh.NewGroup(huhFields...).Title(tabName(tab))
			groups = append(groups, g)
		}
	}

	form := huh.NewForm(groups...).
		WithTheme(groveHuhTheme()).
		WithWidth(width).
		WithShowHelp(true).
		WithShowErrors(true)

	// Override quit keymap — we handle escape ourselves in the overlay
	km := huh.NewDefaultKeyMap()
	km.Quit.SetEnabled(false)
	form.WithKeyMap(km)

	return form, vals
}

// buildConfigHuhField creates a Huh field from a ConfigField.
func buildConfigHuhField(f *ConfigField, vals *configFormValues) huh.Field {
	switch f.Type {
	case ConfigBool:
		b := f.Value == "true"
		vals.bools[f.Key] = &b
		return huh.NewConfirm().
			Key(f.Key).
			Title(f.Label).
			Description(f.Description).
			Affirmative("true").
			Negative("false").
			Value(&b)

	case ConfigEnum:
		vals.strings[f.Key] = &f.Value
		return huh.NewSelect[string]().
			Key(f.Key).
			Title(f.Label).
			Description(f.Description).
			Options(huh.NewOptions(f.Options...)...).
			Value(&f.Value)

	case ConfigList:
		vals.strings[f.Key] = &f.Value
		placeholder := "comma-separated values"
		if f.Placeholder != "" {
			placeholder = f.Placeholder
		}
		return huh.NewInput().
			Key(f.Key).
			Title(f.Label).
			Description(f.Description).
			Placeholder(placeholder).
			Value(&f.Value)

	case ConfigString:
		vals.strings[f.Key] = &f.Value
		placeholder := ""
		if f.Placeholder != "" {
			placeholder = f.Placeholder
		}
		return huh.NewInput().
			Key(f.Key).
			Title(f.Label).
			Description(f.Description).
			Placeholder(placeholder).
			Value(&f.Value)

	default:
		return nil
	}
}
