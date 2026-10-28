package main

import (
	"embed"
	"log"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// contentSecurityPolicy is defence in depth for the embedded UI: the app
// escapes everything it renders, but a CSP keeps a future escaping mistake
// from turning into script execution. Inline styles are still required by
// the existing markup; scripts are not.
func contentSecurityPolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' data: https://fonts.gstatic.com; "+
				"img-src 'self' data:; "+
				"connect-src 'self' ipc: http://ipc.localhost; "+
				"object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func main() {
	app, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:            "Exposely",
		Width:            1040,
		Height:           700,
		MinWidth:         900,
		MinHeight:        620,
		BackgroundColour: &options.RGBA{R: 247, G: 244, B: 236, A: 1},
		AssetServer:      &assetserver.Options{Assets: assets, Middleware: contentSecurityPolicy},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
