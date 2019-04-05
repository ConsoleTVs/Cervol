package pulsar

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"github.com/kabukky/httpscerts"
	"github.com/pulsar-go/pulsar/config"
	"github.com/pulsar-go/pulsar/database"
	"github.com/pulsar-go/pulsar/request"
	"github.com/pulsar-go/pulsar/router"
)

// fileExists determines if a file exists in a given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// debugHandler is responsible for each http handler in debug mode.
func developmentHandler(route *router.Route) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(writer http.ResponseWriter, req *http.Request, params httprouter.Params) {
		log.Printf("[PULSAR] Request %s\n", req.URL)
		response := route.Handler(&request.HTTP{Request: req, Params: params})
		response.Handle(writer)
	}
}

// productionHandler is responsible for each http handler in debug mode.
func productionHandler(route *router.Route) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(writer http.ResponseWriter, req *http.Request, params httprouter.Params) {
		response := route.Handler(&request.HTTP{Request: req, Params: params})
		response.Handle(writer)
	}
}

// RegisterRoutes registers the routes.
func RegisterRoutes(mux *httprouter.Router, r *router.Router) {
	// Register the routes.
	var handler func(*router.Route) func(http.ResponseWriter, *http.Request, httprouter.Params)

	if config.Settings.Server.Development {
		handler = developmentHandler
	} else {
		handler = productionHandler
	}

	for _, route := range r.Routes {
		switch route.Method {
		case request.GetRequest:
			mux.GET(route.URI, handler(&route))
		case request.HeadRequest:
			mux.HEAD(route.URI, handler(&route))
		case request.PostRequest:
			mux.POST(route.URI, handler(&route))
		case request.PutRequest:
			mux.PUT(route.URI, handler(&route))
		case request.PatchRequest:
			mux.PATCH(route.URI, handler(&route))
		case request.DeleteRequest:
			mux.DELETE(route.URI, handler(&route))
		}
	}

	// Register his childs.
	for _, route := range r.Childs {
		RegisterRoutes(mux, route)
	}
}

// Serve starts the server.
func Serve() error {
	router := &router.Routes
	mux := httprouter.New()
	// Register the application routes.
	RegisterRoutes(mux, router)
	// Set the address of the server.
	address := config.Settings.Server.Host + ":" + config.Settings.Server.Port

	generateSSLCertificate(address)

	// Set the database configuration
	database.Open(&config.Settings.Database)
	defer database.DB.Close()

	// Migrate if nessesary
	if config.Settings.Database.AutoMigrate {
		database.DB.AutoMigrate(database.Models...)
	}

	if config.Settings.Server.Development {
		fmt.Println("-----------------------------------------------------")
		fmt.Println("|                                                   |")
		fmt.Println("|  P U L S A R                                      |")
		fmt.Println("|  Go Web framework                                 |")
		fmt.Println("|                                                   |")
		fmt.Println("|  Erik Campobadal <soc@erik.cat>                   |")
		fmt.Println("|  Krishan König <krishan.koenig@googlemail.com>    |")
		fmt.Println("|                                                   |")
		fmt.Println("-----------------------------------------------------")
		fmt.Println()
	}

	if config.Settings.HTTPS.Enabled {
		if config.Settings.Server.Development {
			fmt.Printf("Creating a HTTP/2 server with TLS on %s\n", address)
			fmt.Printf("Certificate: %s\nKey: %s\n\n", config.Settings.HTTPS.CertFile, config.Settings.HTTPS.KeyFile)
		}
		return http.ListenAndServeTLS(address, config.Settings.HTTPS.CertFile, config.Settings.HTTPS.KeyFile, mux)
	}

	if config.Settings.Server.Development {
		fmt.Printf("Creating a HTTP/1.1 server on %s\n\n", address)
	}

	return http.ListenAndServe(address, mux)
}

// generateSSLCertificate creates an ssl certificate if https is enabled
func generateSSLCertificate(address string) {
	// Generate a SSL certificate if needed.
	if !config.Settings.HTTPS.Enabled {
		return
	}

	err := httpscerts.Check(config.Settings.HTTPS.CertFile, config.Settings.HTTPS.KeyFile)
	if err == nil {
		return
	}

	// If they are not available, generate new ones.
	err = httpscerts.Generate(config.Settings.HTTPS.CertFile, config.Settings.HTTPS.KeyFile, address)
	if err != nil {
		log.Fatal("Unable to create HTTP certificates.")
	}
}
