// Package transfertest 는 다른 영역의 실서버 시험이 시험 파일을 올리는 도우미다. 앱 코드는 쓰지 않는다(화면의 업로드는 svc 대기열을 지난다).
package transfertest

import (
	"fmt"
	"path/filepath"

	"snbapp/internal/approval"
	"snbapp/internal/nbs"
	"snbapp/internal/transfer"
	"snbapp/pb/serverfile"
)

// Upload 는 결재자 창이 필요 없는 계정에서 파일 하나를 대기열 없이 곧바로 올린다.
func Upload(s *nbs.Session, path string) error {
	pre := s.Profile["fm_upfile_mode"] != "1"
	aprv := s.AprvType(false)
	var approvers []string
	switch {
	case aprv == nbs.AprvSelf:
		approvers = []string{s.UserID} // 자가승인
	case pre && aprv != nbs.AprvAuto:
		return fmt.Errorf("이 계정은 업로드할 때 결재자를 정해야 합니다(결재 방식 %d)", aprv)
	}
	resv, kind := approval.UploadFields(false, pre, aprv, len(approvers))
	ctx, cancel := s.Ctx(0)
	defer cancel()
	sum := ""
	if s.Profile["fm_checksum"] != "0" {
		var err error
		if sum, err = transfer.HashFile(ctx, path, nil); err != nil {
			return err
		}
	}
	return transfer.Send(ctx, s.ServerFile(), transfer.UploadSpec{Path: path, Info: &serverfile.FileTransInfo{
		Fpath: filepath.Base(path), ApproverId: approvers, AprvKind: kind, AprvType: aprv,
		AprvDept: s.Info.UserDeptName, Checksum: sum, Resv: resv,
	}}, &transfer.Progress{})
}
