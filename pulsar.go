package pulsar

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/kabukky/httpscerts"
	"github.com/pulsar-go/pulsar/config"
	"github.com/pulsar-go/pulsar/db"
	"github.com/pulsar-go/pulsar/queue"
	"github.com/pulsar-go/pulsar/request"
	"github.com/pulsar-go/pulsar/router"
	"github.com/rs/cors"
)

// fileExists determines if a file exists in a given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// debugHandler is responsible for each http handler in debug mode.
func developmentHandler(route *router.Route) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		log.Printf("[PULSAR] Request %s\n", r.URL)
		req := &request.HTTP{Request: r, Writer: w, Params: ps}
		buff, err := ioutil.ReadAll(req.Request.Body)
		if err != nil {
			log.Printf("[PULSAR] Failed to read the request body\n")
		}
		req.Body = string(buff)
		req.Request.Body = ioutil.NopCloser(bytes.NewBuffer(buff))
		res := route.Handler(req)
		res.Handle(req)
	}
}

// productionHandler is responsible for each http handler in debug mode.
func productionHandler(route *router.Route) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		req := &request.HTTP{Request: r, Writer: w, Params: ps}
		buff, _ := ioutil.ReadAll(req.Request.Body)
		req.Body = string(buff)
		req.Request.Body = ioutil.NopCloser(bytes.NewBuffer(buff))
		res := route.Handler(req)
		res.Handle(req)
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
	for _, element := range r.Routes {
		route := element
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
	for _, element := range r.Childs {
		RegisterRoutes(mux, element)
	}
}

// Serve starts the server.
func Serve() error {
	router := &router.Routes
	mux := httprouter.New()
	// Register the application routes.
	RegisterRoutes(mux, router)
	// Register the CORS
	handler := cors.New(cors.Options{
		AllowedOrigins:     config.Settings.Server.AllowedOrigins,
		AllowedHeaders:     config.Settings.Server.AllowedHeaders,
		AllowedMethods:     config.Settings.Server.AllowedMethods,
		AllowCredentials:   config.Settings.Server.AllowCredentials,
		ExposedHeaders:     config.Settings.Server.ExposedHeaders,
		Debug:              config.Settings.Server.Development,
		OptionsPassthrough: true,
	}).Handler(mux)
	// Set the address of the server.
	address := config.Settings.Server.Host + ":" + config.Settings.Server.Port
	// Generate SSL.
	generateSSLCertificate(address)
	// Set the database configuration
	db.Open()
	defer db.Builder.Close()
	// Migrate if nessesary
	if config.Settings.Database.AutoMigrate {
		db.Builder.AutoMigrate(db.Models...)
	}
	// Configure the queue system.
	routines, err := strconv.ParseInt(config.Settings.Queue.Routines, 10, 32)
	if err != nil {
		log.Fatal(err)
	}
	queue.NewPool(int(routines))
	defer queue.Pool.Release()
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
	if config.Settings.Certificate.Enabled {
		if config.Settings.Server.Development {
			fmt.Printf("Creating a HTTP/2 server with TLS on %s\n", address)
			fmt.Printf("Certificate: %s\nKey: %s\n\n", config.Settings.Certificate.CertFile, config.Settings.Certificate.KeyFile)
		}
		return http.ListenAndServeTLS(address, config.Settings.Certificate.CertFile, config.Settings.Certificate.KeyFile, handler)
	}
	if config.Settings.Server.Development {
		fmt.Printf("Creating a HTTP/1.1 server on %s\n\n", address)
	}
	return http.ListenAndServe(address, handler)
}

// generateSSLCertificate creates an ssl certificate if https is enabled
func generateSSLCertificate(address string) {
	// Generate a SSL certificate if needed.
	if !config.Settings.Certificate.Enabled {
		return
	}

	err := httpscerts.Check(config.Settings.Certificate.CertFile, config.Settings.Certificate.KeyFile)
	if err == nil {
		return
	}

	// If they are not available, generate new ones.
	err = httpscerts.Generate(config.Settings.Certificate.CertFile, config.Settings.Certificate.KeyFile, address)
	if err != nil {
		log.Fatal("Unable to create HTTP certificates.")
	}
}
