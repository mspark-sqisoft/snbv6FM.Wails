package svc

import (
	"snbapp/internal/applog"
	"snbapp/internal/integrity"
	"snbapp/internal/notify"
	pbn "snbapp/pb/notify"
)

// wire 는 영역 사이 연결을 한 곳에 모은다(서비스를 모두 만든 뒤 Services 가 한 번 부른다).
//
// 서버 알림 분배표(원본 receivedNotify): 9001 목록·9002 결재 요청·9004 kind 0~5·6·7·9007 공지의 기본 처리는 log.go 의
// registerDefaults 가 등록했고(결재 알림은 트레이 풍선까지 그쪽이 한다), 9004 kind 4 는 링크 영역이 등록한다.
// 여기서는 어느 영역에도 속하지 않는 것만 잇는다.
func (c *Core) wire() {
	c.Notify().Register(notify.SvcIntegrity, notify.AnyKind, func(*pbn.NotifyMessage) {
		c.log.Info("프로그램의 무결성을 확인합니다.", "category", applog.CatAuth)
		c.CheckIntegrity(integrity.KindAdmin)
	})
}
