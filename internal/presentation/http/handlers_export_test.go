package httpserver

import "testing"

func TestSpreadsheetCell_GivenFormulaLikeTitle_EscapesIt(t *testing.T) {
	for _, title := range []string{"=SUM(1,2)", " +cmd", "\t@user"} {
		if got := spreadsheetCell(title); got != "'"+title {
			t.Fatalf("spreadsheetCell(%q) = %q", title, got)
		}
	}
	if got := spreadsheetCell("Ordinary title"); got != "Ordinary title" {
		t.Fatalf("ordinary title changed: %q", got)
	}
}
