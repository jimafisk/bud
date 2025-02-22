package budsvr

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"

	"github.com/livebud/bud/framework"
	"github.com/livebud/bud/internal/pubsub"
	"github.com/livebud/bud/internal/sig"
	"github.com/livebud/bud/package/js"
	"github.com/livebud/bud/package/log"
	"golang.org/x/sync/errgroup"
)

// New development server
func New(budln net.Listener, bus pubsub.Client, flag *framework.Flag, fsys fs.FS, log log.Log, vm js.VM) *Server {
	return &Server{
		ln: budln,
		s: &http.Server{
			Addr:    budln.Addr().String(),
			Handler: newHandler(flag, fsys, bus, log, vm),
		},
		eg: new(errgroup.Group),
	}
}

type Server struct {
	ln net.Listener
	s  *http.Server
	eg *errgroup.Group
}

// Shutdown the server gracefully unless we receive an interrupt signal, then
// force an immediate shutdown.
func (s *Server) shutdown() error {
	forceCtx := sig.Trap(context.Background(), os.Interrupt)
	return s.s.Shutdown(forceCtx)
}

func (s *Server) serve() error {
	return s.s.Serve(s.ln)
}

// Address returns the listener's address
func (s *Server) Address() string {
	return s.ln.Addr().String()
}

// Start the server in the background
func (s *Server) Start(ctx context.Context) {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		<-ctx.Done()
		return s.shutdown()
	})
	eg.Go(func() error {
		return s.serve()
	})
	s.eg = eg
}

// Wait for the server to shutdown
func (s *Server) Wait() error {
	if err := s.eg.Wait(); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

// Listen for requests. If the context is cancelled, the server will begin a
// graceful shutdown
func (s *Server) Listen(ctx context.Context) error {
	s.Start(ctx)
	return s.Wait()
}

// Shutdown the server gracefully
func (s *Server) Close() error {
	if err := s.shutdown(); err != nil {
		return err
	}
	return s.Wait()
}
