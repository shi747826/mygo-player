package plugins

import "mygo-player/pluginapi"

type wireError = pluginapi.Error

var (
	ErrClosed       = pluginapi.ErrClosed
	ErrProtocol     = pluginapi.ErrProtocol
	ErrUnsupported  = pluginapi.ErrUnsupported
	ErrWorkerExited = &pluginapi.Error{Code: "worker_exited"}
)

func encodeError(err error) *wireError { return pluginapi.EncodeError(err) }
func decodeError(w *wireError) error {
	if w == nil {
		return nil
	}
	return w
}
