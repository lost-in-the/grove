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
		t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(cs.SurfaceBg).Background(cs.Primary)
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

// configFormValues holds the bool bindings the forms write to; string,
// enum, and list fields bind straight to their ConfigField.Value.
type configFormValues struct {
	bools map[string]*bool
}

func newConfigFormValues() *configFormValues {
	return &configFormValues{bools: map[string]*bool{}}
}

// buildSectionForm builds one section's form. Grove draws the section tabs,
// the keys, and the errors, so huh shows neither help nor errors, and a
// fixed height keeps every section the same size (spec 11). A height of 0
// leaves huh's natural height.
func buildSectionForm(fields []ConfigField, vals *configFormValues, width, height int) *huh.Form {
	var huhFields []huh.Field
	for i := range fields {
		if field := buildConfigHuhField(&fields[i], vals); field != nil {
			huhFields = append(huhFields, field)
		}
	}

	km := huh.NewDefaultKeyMap()
	km.Quit.SetEnabled(false) // grove handles esc
	km.Select.Filter.SetEnabled(false)

	return huh.NewForm(huh.NewGroup(huhFields...)).
		WithTheme(groveHuhTheme()).
		WithKeyMap(km).
		WithWidth(width).
		WithHeight(height).
		WithShowHelp(false).
		WithShowErrors(false)
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
		return huh.NewSelect[string]().
			Key(f.Key).
			Title(f.Label).
			Description(f.Description).
			Options(huh.NewOptions(f.Options...)...).
			Value(&f.Value)

	case ConfigList:
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
		return huh.NewInput().
			Key(f.Key).
			Title(f.Label).
			Description(f.Description).
			Placeholder(f.Placeholder).
			Value(&f.Value)

	default:
		return nil
	}
}
