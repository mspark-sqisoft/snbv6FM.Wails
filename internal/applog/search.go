package applog

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"
)

// Query 는 [로그] 화면의 조회 조건이다(LOG-038).
type Query struct {
	Level    string // "" 전부, "INFO" 정확히 같은 수준만(원본), "INFO+" 이 수준 이상([Wails] 더함)
	Category string // "" 전부, 그 밖은 접두사("snb.transfer" 는 snb.transfer.upload 도)
	Word     string // 메시지 칸만, 대소문자 무시
	Max      int    // 돌려줄 최대 줄 수(넘으면 마지막 Max 줄). 0 이면 제한 없음
}

// Result 는 조회 결과다. Lines 는 파일 순서(오래된 것 먼저)이고 No 는 맞은 줄의 1부터 순번이다.
type Result struct {
	Path    string
	Lines   []Line
	Nos     []int
	Total   int  // 파일의 줄 수
	Matched int  // 맞은 줄 수(Max 로 잘리기 전)
	Missing bool // 그날 파일이 없다
	Elapsed time.Duration
}

var levelRank = map[string]int{"DEBUG": 0, "INFO": 1, "WARN": 2, "ERROR": 3, "FATAL": 4}

// Search 는 로그 폴더의 date 하루치(yyyy-MM-dd 또는 yyyyMMdd) 파일을 한 번 읽으며 거른다(LOG-041·042).
// 새 형식이 아닌 줄은 건너뛴다. 파일이 없으면 Missing.
func Search(date string, q Query) (*Result, error) {
	return searchIn(Dir(), date, q)
}

func searchIn(dir, date string, q Query) (*Result, error) {
	start := time.Now()
	d, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		if d, err = time.ParseInLocation("20060102", date, time.Local); err != nil {
			return nil, errors.New("날짜 형식이 아닙니다: " + date)
		}
	}
	r := &Result{Path: FilePath(dir, d)}
	f, err := os.Open(r.Path)
	if errors.Is(err, fs.ErrNotExist) {
		r.Missing = true
		r.Elapsed = time.Since(start)
		return r, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	lv, atLeast := strings.CutSuffix(strings.ToUpper(strings.TrimSpace(q.Level)), "+")
	minRank, rankOK := levelRank[lv]
	word := strings.ToLower(q.Word)
	cat := q.Category

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 256*1024), 4*1024*1024)
	for sc.Scan() {
		r.Total++
		l, ok := ParseLine(sc.Text())
		if !ok {
			continue
		}
		if lv != "" {
			if atLeast && rankOK {
				if rk, ok := levelRank[l.Level]; !ok || rk < minRank {
					continue
				}
			} else if l.Level != lv {
				continue
			}
		}
		if cat != "" && l.Category != cat && !strings.HasPrefix(l.Category, cat+".") {
			continue
		}
		if word != "" && !strings.Contains(strings.ToLower(l.Message), word) {
			continue
		}
		r.Matched++
		r.Lines = append(r.Lines, l)
		r.Nos = append(r.Nos, r.Matched)
		if q.Max > 0 && len(r.Lines) >= 2*q.Max { // 마지막 Max 줄만 남긴다(메모리 상한)
			r.Lines = append(r.Lines[:0:0], r.Lines[len(r.Lines)-q.Max:]...)
			r.Nos = append(r.Nos[:0:0], r.Nos[len(r.Nos)-q.Max:]...)
		}
	}
	if q.Max > 0 && len(r.Lines) > q.Max {
		r.Lines = r.Lines[len(r.Lines)-q.Max:]
		r.Nos = r.Nos[len(r.Nos)-q.Max:]
	}
	r.Elapsed = time.Since(start)
	return r, sc.Err()
}
