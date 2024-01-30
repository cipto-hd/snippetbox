package main

import (
	"net/http"
	"path/filepath"

	"github.com/julienschmidt/httprouter"
	"github.com/justinas/alice"

	"github.com/cipto-hd/snippetbox/ui"
)

// Update the signature for the routes() method so that it returns a
// http.Handler instead of *http.ServeMux.
func (app application) routes() http.Handler {
	// mux := http.NewServeMux()
	router := httprouter.New()

	// Create a handler function which wraps our notFound() helper, and then
	// assign it as the custom handler for 404 Not Found responses. You can also
	// set a custom handler for 405 Method Not Allowed responses by setting
	// router.MethodNotAllowed in the same way too.
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.notFound(w)
	})

	fileServer := http.FileServer(neuteredFileSystem{http.FS(ui.Files)})

	// mux.Handle("/static/", http.StripPrefix("/static", fileServer))
	// Our static files are contained in the "static" folder of the ui.Files
	// embedded filesystem. So, for example, our CSS stylesheet is located at
	// "static/css/main.css". This means that we now longer need to strip the
	// prefix from the request URL -- any requests that start with /static/ can
	// just be passed directly to the file server and the corresponding static
	// file will be served (so long as it exists).
	router.Handler(http.MethodGet, "/static/*filepath", fileServer)

	// Unprotected application routes using the "dynamic" middleware chain.
	dynamic := alice.New(app.sessionManager.LoadAndSave, noSurf, app.authenticate)

	addAliceChainToRoutes(router, dynamic, []MethodPathHandlerFunc{
		{
			Method:      http.MethodGet,
			Path:        "/",
			HandlerFunc: app.showHome,
		},
		{
			Method:      http.MethodGet,
			Path:        "/snippet/view/:id",
			HandlerFunc: app.showSnippetView,
		},
		{
			Method:      http.MethodGet,
			Path:        "/user/signup",
			HandlerFunc: app.showUserSignup,
		},
		{
			Method:      http.MethodPost,
			Path:        "/user/signup",
			HandlerFunc: app.doUserSignup,
		},
		{
			Method:      http.MethodGet,
			Path:        "/user/login",
			HandlerFunc: app.showUserLogin,
		},
		{
			Method:      http.MethodPost,
			Path:        "/user/login",
			HandlerFunc: app.doUserLogin,
		},
	})

	// Protected (authenticated-only) application routes, using a new "protected"
	// middleware chain which includes the requireAuthentication middleware.
	protected := dynamic.Append(app.requireAuthentication)
	addAliceChainToRoutes(router, protected, []MethodPathHandlerFunc{
		{
			Method:      http.MethodGet,
			Path:        "/snippet/create",
			HandlerFunc: app.showSnippetCreate,
		},
		{
			Method:      http.MethodPost,
			Path:        "/snippet/create",
			HandlerFunc: app.doSnippetCreate,
		},
		{
			Method:      http.MethodPost,
			Path:        "/user/logout",
			HandlerFunc: app.doUserLogout,
		},
	})

	// Pass the servemux as the 'next' parameter to the secureHeaders middleware.
	// Because secureHeaders is just a function, and the function returns a
	// http.Handler we don't need to do anything else.
	// Wrap the existing chain with the logRequest middleware.
	// return app.recoverPanic(app.logRequest(secureHeaders(mux)))

	// Create a middleware chain containing our 'standard' middleware
	// which will be used for every request our application receives.
	standard := alice.New(app.recoverPanic, app.logRequest, secureHeaders)
	// Return the 'standard' middleware chain followed by the servemux.
	return standard.Then(router)
}

type neuteredFileSystem struct {
	fs http.FileSystem
}

func (nfs neuteredFileSystem) Open(path string) (http.File, error) {

	f, err := nfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	s, _ := f.Stat()
	if s.IsDir() {
		index := filepath.Join(path, "index.html")
		if _, err := nfs.fs.Open(index); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}

			return nil, err
		}
	}

	return f, nil
}

type MethodPathHandlerFunc struct {
	Method      string
	Path        string
	HandlerFunc http.HandlerFunc
}

func addAliceChainToRoutes(router *httprouter.Router, ac alice.Chain, mphArr []MethodPathHandlerFunc) {
	for _, mph := range mphArr {
		router.Handler(mph.Method, mph.Path, ac.ThenFunc(mph.HandlerFunc))
	}
}
