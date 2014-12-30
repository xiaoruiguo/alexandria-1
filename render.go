/*
 * Alexandria CMDB - Open source common.management database
 * Copyright (C) 2014  Ryan Armstrong <ryan@cavaliercoder.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 * package controllers
 */
package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2"
	"io"
	"log"
	"net/http"
)

func Handle(res http.ResponseWriter, req *http.Request, err error) bool {
	// Is this a generic Mongo Not Found error?
	if err == mgo.ErrNotFound {
		ErrNotFound(res, req)
		return true
	}

	// Is this a Mongo error?
	mgoErr, ok := err.(*mgo.LastError)
	if ok {
		switch mgoErr.Code {
		case 11000: // Duplicate key insertion
			ErrConflict(res, req)
			return true
		}
	}

	// Unknown error
	if err != nil {
		ErrUnknown(res, req, err)
		return true
	}

	return false
}

func ErrUnknown(res http.ResponseWriter, req *http.Request, err error) {
	log.Printf("ERROR: %#v", err)
	res.WriteHeader(http.StatusInternalServerError)
	res.Write([]byte("500 Internal server error"))
}

func ErrNotFound(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotFound)
	res.Write([]byte("404 Resource not found"))
}

func ErrConflict(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusConflict)
	res.Write([]byte("409 Conflict"))
}

func ErrBadRequest(res http.ResponseWriter, req *http.Request, err error) {
	log.Printf("Bad request: %s", err.Error())
	res.WriteHeader(http.StatusBadRequest)
	res.Write([]byte(fmt.Sprintf("400 Bad request\n%s", err.Error())))
}

func RenderCreated(res http.ResponseWriter, req *http.Request, url string) {
	res.Header().Set("Location", url)
	Render(res, req, http.StatusCreated, "")
}

func Render(res http.ResponseWriter, req *http.Request, status int, v interface{}) {
	format := req.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		RenderJson(res, req, status, v)

	default:
		log.Panic(fmt.Sprintf("Unsupported output format: %s", format))
	}
}

func RenderJson(res http.ResponseWriter, req *http.Request, status int, v interface{}) {
	if v == nil {
		v = new(struct{})
	}

	var err error
	var data []byte
	if req.URL.Query().Get("pretty") == "true" {
		data, err = json.MarshalIndent(v, "", "    ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		log.Panic(err)
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	res.Write(data)
}

func Bind(req *http.Request, v interface{}) error {
	if req.Body != nil {
		defer req.Body.Close()

		err := json.NewDecoder(req.Body).Decode(v)

		if err != nil && err != io.EOF {
			return err
		}
	}

	return nil
}

func GetPathVar(req *http.Request, name string) string {
	vars := mux.Vars(req)
	result := vars[name]

	if name == "" {
		log.Panic(fmt.Sprintf("No such variable declared: %s", name))
	}

	return result
}

func GetCmdbBackend(req *http.Request) *mgo.Database {
	// Get authentication context
	auth := GetAuthContext(req)
	if auth == nil {
		log.Panic("A CMDB was requested without valid authentication")
		return nil
	}

	// Get requested CMDB name
	name := GetPathVar(req, "cmdb")

	// Get the CMDB details
	cmdb, ok := auth.Tenant.Cmdbs[name]
	if !ok {
		return nil
	}

	// Return the backend database
	db := DbConnect()
	return db.DB(cmdb.GetBackendName())
}
