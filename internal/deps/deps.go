//go:build deps

// Package deps 는 영역 담당이 쓸 의존을 go.mod 에 붙잡아 둔다. wails3 build 가 go mod tidy 로 안 쓰는 의존을 지우기 때문이다
// (tidy 는 빌드 태그와 무관하게 모든 파일을 본다). 실제로 쓰기 시작한 패키지는 여기서 지워도 된다.
package deps

import (
	_ "github.com/bodgit/sevenzip"         // 7z 검사(정책)
	_ "github.com/klauspost/compress/zstd" // 압축 형식(정책)
	_ "github.com/ulikunitz/xz"            // xz 검사(정책)
	_ "github.com/xuri/excelize/v2"        // 엑셀 저장(파일·내역)
	_ "github.com/yeka/zip"                // 암호 zip(정책)
	_ "golang.org/x/image/bmp"             // 클립보드 그림(클립보드)
	_ "golang.org/x/net/html"              // 클립보드 HTML(클립보드)
	_ "golang.org/x/sys/windows/registry"  // 옛 설정 옮기기(메인)
	_ "golang.org/x/text/encoding/korean"  // EUC-KR(여러 영역)
)
