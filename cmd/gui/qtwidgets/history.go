package qtwidgets

import (
	"github.com/therecipe/qt/widgets"
	"strings"
)

var WebHistory *widgets.QTextEdit
var numberLines = 2000
var textHistory string

func AddNewHistoryItem(item string) {
	if item == "" || item == "\n" {
		return
	}
	t := item + "\n"
	t += textHistory

	if strings.Count(t, "\n") > numberLines {
		tl := strings.Split(t, "\n")
		t = strings.Join(tl[:numberLines], "\n")
	}
	textHistory = t
}

func ShowHistoryPage() *widgets.QTabWidget {
	// create a regular widget
	// give it a QVBoxLayout
	// and make it the central widget of the window
	widget := widgets.NewQTabWidget(nil)
	widget.SetLayout(widgets.NewQVBoxLayout())

	UpdateTextButton := widgets.NewQPushButton2("Update", nil)
	UpdateTextButton.ConnectClicked(func(bool) {
		WebHistory.SetText(textHistory)
		cursor := WebHistory.TextCursor()
		cursor.MovePosition(11, 0, 0)
	})
	widget.Layout().AddWidget(UpdateTextButton)

	WebHistory = widgets.NewQTextEdit(nil)
	widget.Layout().AddWidget(WebHistory)

	return widget
}
