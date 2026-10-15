package logs

import (
	"log/slog"

	"github.com/rotisserie/eris"
)

func ErisError(err error) slog.Attr {
	return slog.Any("err", eris.ToJSON(err, true))
}
