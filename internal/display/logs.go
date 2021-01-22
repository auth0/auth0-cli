package display

import (
	"fmt"
	"time"

	"github.com/auth0/auth0-cli/internal/ansi"
	"gopkg.in/auth0.v5/management"
)

func (r *Renderer) LogList(logs []*management.Log) {
	for _, c := range logs {
		// TODO: Info/Warn/Error based on type
		r.Infof(fmt.Sprintf("%s\t%s\t%s",
			ansi.Faint(c.GetDate().Format(time.RFC3339)),
			ansi.Faint(c.TypeName()),
			ansi.Faint(*c.ClientName),
		))
	}
}
