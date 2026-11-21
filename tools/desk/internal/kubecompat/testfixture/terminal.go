package testfixture

import (
	"context"
	"encoding/json"
	"io"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"net/http"
)

// TerminalHandler connects one retained fixture session to an operator's streams.
type TerminalHandler func(context.Context, string, io.Reader, io.Writer, io.Reader) error

func (s *Server) attachTerminal(req *http.Request, conn httpstream.Connection, role string, streams map[string]httpstream.Stream) {
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	go func() {
		select {
		case <-conn.CloseChan():
			cancel()
		case <-ctx.Done():
		}
	}()
	err := s.Terminal(ctx, role, streams[core.StreamTypeStdin], streams[core.StreamTypeStdout], streams[core.StreamTypeResize])
	if status := streams[core.StreamTypeError]; status != nil {
		result := meta.Status{Status: meta.StatusSuccess}
		if err != nil {
			result.Status = meta.StatusFailure
			result.Message = err.Error()
		}
		_ = json.NewEncoder(status).Encode(result)
		_ = status.Close()
	}
}
