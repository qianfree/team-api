package common

import "strings"

// ---------------------------------------------------------------------------
// JSON path（"data.videos[0].url"）
// ---------------------------------------------------------------------------

// GetJSONPath 按点分 + [n] 路径取值。数组段的纯数字 key 兼容为下标
// （`results.0.url` ≡ `results[0].url`——AI 生成插件的常见笔误形态，
// 取不到值会让转存/交付整链静默失败，此处直接兼容掉）。
func GetJSONPath(data map[string]any, path string) any {
	cur := any(data)
	for _, seg := range splitPath(path) {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg.key]
			if !ok {
				return nil
			}
			cur = v
		case []any:
			idx := seg.index
			if idx < 0 {
				idx = seg.numericIndex()
			}
			if idx < 0 || idx >= len(node) {
				return nil
			}
			cur = node[idx]
		default:
			return nil
		}
	}
	return cur
}

// SetJSONPath 按路径改写值（路径必须已存在——产物声明指向 taskData 已有字段）。
// 数组段的纯数字 key 同 GetJSONPath 兼容为下标。
func SetJSONPath(data map[string]any, path string, value any) bool {
	segs := splitPath(path)
	if len(segs) == 0 {
		return false
	}
	parent := any(data)
	for i := 0; i < len(segs)-1; i++ {
		switch node := parent.(type) {
		case map[string]any:
			v, ok := node[segs[i].key]
			if !ok {
				return false
			}
			parent = v
		case []any:
			idx := segs[i].index
			if idx < 0 {
				idx = segs[i].numericIndex()
			}
			if idx < 0 || idx >= len(node) {
				return false
			}
			parent = node[idx]
		default:
			return false
		}
	}
	last := segs[len(segs)-1]
	switch node := parent.(type) {
	case map[string]any:
		if _, ok := node[last.key]; !ok {
			return false
		}
		node[last.key] = value
		return true
	case []any:
		idx := last.index
		if idx < 0 {
			idx = last.numericIndex()
		}
		if idx < 0 || idx >= len(node) {
			return false
		}
		node[idx] = value
		return true
	default:
		return false
	}
}

type pathSeg struct {
	key   string
	index int
}

// numericIndex 段 key 为纯数字时返回其值（否则 -1）：点分形态的数组下标兼容
// （`results.0.url` 的 "0" 段）。map 上的数字字符串键不受影响——本方法只在
// 节点为 []any 的分支被调用。
func (s pathSeg) numericIndex() int {
	if s.key == "" {
		return -1
	}
	n := 0
	for _, c := range s.key {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func splitPath(path string) []pathSeg {
	var segs []pathSeg
	for _, part := range strings.Split(path, ".") {
		if part == "" {
			continue
		}
		// 处理 "videos[0]" 形态
		bracket := strings.Index(part, "[")
		if bracket < 0 {
			segs = append(segs, pathSeg{key: part, index: -1})
			continue
		}
		if bracket > 0 {
			segs = append(segs, pathSeg{key: part[:bracket], index: -1})
		}
		for bracket >= 0 {
			end := strings.Index(part[bracket:], "]")
			if end < 0 {
				return segs
			}
			idx := 0
			numStr := part[bracket+1 : bracket+end]
			for _, c := range numStr {
				if c < '0' || c > '9' {
					return segs
				}
				idx = idx*10 + int(c-'0')
			}
			segs = append(segs, pathSeg{key: "", index: idx})
			next := strings.Index(part[bracket+end:], "[")
			if next < 0 {
				break
			}
			bracket = bracket + end + next
		}
	}
	return segs
}
