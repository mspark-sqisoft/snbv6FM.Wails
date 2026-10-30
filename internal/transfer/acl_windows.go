package transfer

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// inheritACL 은 옮겨 온 파일·폴더가 지금 자리의 권한을 따르게 한다. 같은 볼륨 안의 이동은 원래 자리의 권한(ACL)을 그대로 달고 와서, 앱 임시 폴더에서
// 옮긴 것은 받을 폴더의 권한을 따르지 않는다(실측: C:\Users\Public 으로 옮긴 폴더가 사용자 프로필 쪽 권한을 유지했다). 폴더면 안쪽까지 다시 계산된다.
// 자리가 물려줄 항목이 없으면(폴더 자신에게만 건 권한) 그대로 둔다 ― 호출은 성공하면서 빈 목록으로 확정돼 소유자도 못 연다(실측).
// 권한이 없는 디스크(FAT 등)에서는 실패하고, 그러면 이동한 그대로 둔다.
func inheritACL(path string) {
	if !inheritable(path) {
		return
	}
	// 항목 없는 ACL 은 SDDL 로 만든다. ACLFromEntries(nil, nil) 은 NULL 을 내는데, NULL DACL 은 모든 사람에게 전체 권한이다.
	sd, err := windows.SecurityDescriptorFromString("D:")
	if err != nil {
		return
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return
	}
	windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

// inheritable 은 path 의 부모가 path 에, 폴더면 그 안쪽 파일까지 물려줄 허용 항목을 가졌는지 본다.
func inheritable(path string) bool {
	st, err := os.Lstat(path)
	if err != nil {
		return false
	}
	sd, err := windows.GetNamedSecurityInfo(filepath.Dir(path), windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return false
	}
	need := uint8(windows.OBJECT_INHERIT_ACE)
	if st.IsDir() {
		need |= windows.CONTAINER_INHERIT_ACE
	}
	var got uint8
	for i := range uint32(acl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(acl, i, &ace) != nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}
		if st.IsDir() && ace.Header.AceFlags&windows.NO_PROPAGATE_INHERIT_ACE != 0 { // 폴더까지만 내려가고 안쪽 파일에는 안 간다
			continue
		}
		got |= ace.Header.AceFlags
	}
	return got&need == need
}
