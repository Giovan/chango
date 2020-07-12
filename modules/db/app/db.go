// Copyright (c) 2012-2016 The Chango Framework Authors, All rights reserved.
// Chango Framework source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

// Package db module configures a database connection for the application.
//
// Developers use this module by importing and calling db.Init().
// A "Transactional" controller type is provided as a way to import interceptors
// that manage the transaction
//
// In particular, a transaction is begun before each request and committed on
// success.  If a panic occurred during the request, the transaction is rolled
// back.  (The application may also roll the transaction back itself.)
package db

import (
	"database/sql"

	"github.com/giovan/chango"
)

// Database connection variables
var (
	Db     *sql.DB
	Driver string
	Spec   string
)

// Init method used to initialize DB module on `OnAppStart`
func Init() {
	// Read configuration.
	var found bool
	if Driver, found = chango.Config.String("db.driver"); !found {
		chango.changoLog.Fatal("db.driver not configured")
	}
	if Spec, found = chango.Config.String("db.spec"); !found {
		chango.changoLog.Fatal("db.spec not configured")
	}

	// Open a connection.
	var err error
	Db, err = sql.Open(Driver, Spec)
	if err != nil {
		chango.changoLog.Fatal("Open database connection error", "error", err, "driver", Driver, "spec", Spec)
	}

	chango.OnAppStop(func() {
		chango.changoLog.Info("Closing the database (from module)")
		if err := Db.Close(); err != nil {
			chango.AppLog.Error("Failed to close the database", "error", err)
		}
	})
}

// Transactional definition for database transaction
type Transactional struct {
	*chango.Controller
	Txn *sql.Tx
}

// Begin a transaction
func (c *Transactional) Begin() chango.Result {
	txn, err := Db.Begin()
	if err != nil {
		panic(err)
	}
	c.Txn = txn
	return nil
}

// Rollback if it's still going (must have panicked).
func (c *Transactional) Rollback() chango.Result {
	if c.Txn != nil {
		if err := c.Txn.Rollback(); err != nil {
			if err != sql.ErrTxDone {
				panic(err)
			}
		}
		c.Txn = nil
	}
	return nil
}

// Commit the transaction.
func (c *Transactional) Commit() chango.Result {
	if c.Txn != nil {
		if err := c.Txn.Commit(); err != nil {
			if err != sql.ErrTxDone {
				panic(err)
			}
		}
		c.Txn = nil
	}
	return nil
}

func init() {
	chango.InterceptMethod((*Transactional).Begin, chango.BEFORE)
	chango.InterceptMethod((*Transactional).Commit, chango.AFTER)
	chango.InterceptMethod((*Transactional).Rollback, chango.FINALLY)
}
