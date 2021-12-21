////////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 Privategrity Corporation                                   /
//                                                                             /
// All rights reserved.                                                        /
////////////////////////////////////////////////////////////////////////////////

// Handles the high level storage API.
// This layer merges the business logic layer and the database layer

package storage

import (
	"git.xx.network/elixxir/user-reporting/messages"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
)

// Params for creating a storage object
type Params struct {
	Username string
	Password string
	DBName   string
	Address  string
	Port     string
}

// Storage struct interfaces with the API for the storage layer
type Storage struct {
	// Stored Database interface
	database
}

// NewStorage creates a new Storage object wrapping a database interface
// Returns a Storage object, and error
func NewStorage(params Params) (*Storage, error) {
	db, err := newDatabase(params.Username, params.Password, params.DBName, params.Address, params.Port)
	storage := &Storage{db}
	return storage, err
}

func (s *Storage) StoreReport(r *messages.Report) error {
	reported, err := s.AddOrIncrementReportedUser(r.ReportedId)
	if err != nil {
		err = errors.WithMessage(err, "Failed to add or increment reported user")
		jww.INFO.Println(err)
		return err
	}
	report := &Report{
		Reporter:         r.ReporterId,
		Reported:         *reported,
		ReportedMessages: []ReportedMessage{},
	}

	for _, msg := range r.Messages {
		report.ReportedMessages = append(report.ReportedMessages, ReportedMessage{Contents: msg})
	}

	err = s.InsertReport(report)
	if err != nil {
		err = errors.WithMessage(err, "Failed to insert report in storage")
		jww.INFO.Println(err)
		return err
	}
	return nil
}
