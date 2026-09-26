package httpserver

import (
	"errors"
	"github.com/afonsocosta/visto/internal/application/pagination"
	"net/http"
	"strings"
)

func writePageError(w http.ResponseWriter, err error, serverMessage string) {
	if errors.Is(err, pagination.ErrInvalidCursor) || errors.Is(err, pagination.ErrInvalidLimit) || strings.HasPrefix(err.Error(), "invalid ") {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, serverMessage)
}
