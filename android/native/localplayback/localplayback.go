// Package localplayback allows the controlled Android app page to play media
// served by an embedded HTTP server on localhost. Importing this package lets
// MyGo discover and build its Kotlin Android library, including its manifest.
package localplayback

// Controller configures app WebViews on Android. Other platforms are unsupported.
//
//mygo:native
type Controller interface {
	// EnableLocalPlayback enables mixed content only on WebViews currently at
	// https://mygo.localhost, and returns their count. Calling it again is safe.
	// It resets other WebViews to the default mixed-content policy and fails
	// when no trusted app WebView exists. Call after the app page finishes loading.
	//
	//mygo:mainthread
	EnableLocalPlayback() (int, error)

	// ConvertCover decodes the image at path and writes a JPEG next to it,
	// returning the new path. Android can decode formats the Go side cannot,
	// such as the HEIF covers some sources serve. Call it off the main thread.
	ConvertCover(path string) (string, error)

	// SystemProxy returns the Android proxy URL for the target, or an empty
	// string for a direct connection. Selection may consult a PAC service.
	SystemProxy(target string) (string, error)
}

// EnableLocalPlayback configures all currently loaded, trusted app WebViews.
// Call from a goroutine if the caller is a MyGo lifecycle/UI callback.
func EnableLocalPlayback() (int, error) {
	controller, err := NewController()
	if err != nil {
		return 0, err
	}
	return controller.EnableLocalPlayback()
}

// ConvertCover decodes an image with Android's decoders and writes a JPEG.
// It returns an error where the platform has no decoder for the format.
func ConvertCover(path string) (string, error) {
	controller, err := NewController()
	if err != nil {
		return "", err
	}
	return controller.ConvertCover(path)
}
