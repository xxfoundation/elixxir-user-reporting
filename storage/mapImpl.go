////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package storage

import "encoding/base64"

func (m *MapImpl) InsertReport(r *Report) error {
	m.Lock()
	defer m.Unlock()
	m.reports[m.reportIDSeq] = r

	for _, msg := range r.ReportedMessages {
		m.reportedMessages[m.reportIDSeq] = append(m.reportedMessages[m.reportIDSeq], &msg)
	}

	return nil
}
func (m *MapImpl) AddOrIncrementReportedUser(id []byte) (*ReportedUser, error) {
	b64id := base64.StdEncoding.EncodeToString(id)
	if _, ok := m.reportedUsers[b64id]; ok {
		m.reportedUsers[b64id] += 1
	} else {
		m.reportedUsers[b64id] = 1
	}
	return &ReportedUser{
		ID:      id,
		Reports: m.reportedUsers[b64id],
	}, nil
}
