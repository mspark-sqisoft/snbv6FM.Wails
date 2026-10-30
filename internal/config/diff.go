package config

import (
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
)

// Change 는 설정 키 하나의 바뀜이다(설정 변경 로그 `설정 변경 >>> 키: 이전 → 새`, 원본 SET-089).
type Change struct {
	Key string
	Old string
	New string
}

var secretKey = regexp.MustCompile(`(?i)pw|pass|token|secret`)

// Changes 는 old → cur 에서 바뀐 키를 JSON 이름 순으로 돌려준다. 키 이름이 비밀 단어에 맞으면 값을 가린다.
// 맵(auto·columnWidths)은 통째로 JSON 글자로 비교한다.
func Changes(old, cur Settings) []Change {
	a, b := flat(old), flat(cur)
	var out []Change
	for k, nv := range b {
		ov := a[k]
		if reflect.DeepEqual(ov, nv) {
			continue
		}
		c := Change{Key: k, Old: text(ov), New: text(nv)}
		if secretKey.MatchString(k) {
			c.Old, c.New = "***", "***"
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func flat(s Settings) map[string]any {
	b, _ := json.Marshal(s)
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	return m
}

func text(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
