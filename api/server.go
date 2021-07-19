package api

import (
	"errors"
	"github.com/bulbetski/kvstorage-srv/storage"
	"github.com/bulbetski/kvstorage-srv/utils"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Server struct {
	router  *mux.Router
	storage *storage.Storage
	config  *Config
}

func NewServer(storage *storage.Storage) *Server {
	return &Server{
		router:  mux.NewRouter(),
		storage: storage,
	}
}

func Start(config *Config) error {
	db := storage.New(5*time.Minute, 10*time.Minute, config.DBSize)
	if _, err := os.Stat(config.DBFileName); err == nil {
		if err = db.LoadFile(config.DBFileName); err != nil {
			return err
		}
	}

	srv := NewServer(db)
	//config property is needed to save and load db from client requests (don't know where to put filePath property)
	srv.config = config

	srv.configureRouter()
	srv.PersistDB(config.DBFileName)

	return http.ListenAndServe(config.BindAddr, srv)
}

func (srv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.router.ServeHTTP(w, r)
}

func (srv *Server) configureRouter() {
	srv.router.HandleFunc("/items/{key}/{value}", srv.HandleSet()).Methods("PUT")
	srv.router.HandleFunc("/items/{key}", srv.HandleGet()).Methods("GET")
	srv.router.HandleFunc("/items/", srv.HandleItems()).Methods("GET")
	srv.router.HandleFunc("/items/{key}", srv.HandleDelete()).Methods("DELETE")
	srv.router.HandleFunc("/saveItems", srv.HandleSave()).Methods("GET")
	srv.router.HandleFunc("/loadItems", srv.HandleLoad()).Methods("GET")
}

func (srv *Server) PersistDB(filename string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		srv.storage.SaveFile(filename)
		os.Exit(0)
	}()
}

//TODO:
// check if value from path maps correctly
// add expiration time parameter to path

func (srv *Server) HandleSet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["key"]
		value := vars["value"]

		srv.storage.Set(key, value, storage.DefaultExpiration)
		w.WriteHeader(http.StatusOK)
	}
}

func (srv *Server) HandleGet() http.HandlerFunc {
	type response struct {
		Value interface{} `json:"value"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["key"]

		val, found := srv.storage.Get(key)
		if !found {
			utils.ErrorMessage(w, r, http.StatusNotFound, errors.New("no such key"))
			return
		}
		utils.Respond(w, r, http.StatusOK, response{val})
	}
}

func (srv *Server) HandleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		key := vars["key"]

		deleted := srv.storage.Delete(key)
		if !deleted {
			utils.ErrorMessage(w, r, http.StatusNotFound, errors.New("no such key"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (srv *Server) HandleItems() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := srv.storage.Items()
		utils.Respond(w, r, http.StatusOK, m)
	}
}

func (srv *Server) HandleSave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := srv.storage.SaveFile(srv.config.DBFileName); err != nil {
			utils.ErrorMessage(w, r, http.StatusInternalServerError, errors.New("couldn't save db"))
			return
		}
		//TODO: make meaningful response
		utils.Respond(w, r, http.StatusOK, "")
	}
}

func (srv *Server) HandleLoad() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(srv.config.DBFileName); err == nil {
			if err = srv.storage.LoadFile(srv.config.DBFileName); err != nil {
				utils.ErrorMessage(w, r, http.StatusInternalServerError, errors.New("couldn't load db"))
				return
			}
			utils.Respond(w, r, http.StatusOK, "")
			return
		}
		utils.Respond(w, r, http.StatusNoContent, "")
	}
}
