////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package storage

import (
	"github.com/pkg/errors"
)

func (db *DatabaseImpl) InsertReport(r *Report) error {
	return db.db.Create(r).Error
}
func (db *DatabaseImpl) AddOrIncrementReportedUser(id []byte) (*ReportedUser, error) {
	u := &ReportedUser{
		ID: id,
	}
	err := db.db.FirstOrCreate(*u, "id = ?", id).Error
	if err != nil {
		return nil, errors.WithMessagef(err, "Failed to find or create reporteduser with id %+v", id)
	}
	u.Reports += 1
	return u, db.db.Save(u).Error
}
