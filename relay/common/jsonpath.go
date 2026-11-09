package common

import "strings"

// ---------------------------------------------------------------------------
// JSON path（"data.videos[0].url"）
// ---------------------------------------------------------------------------

// GetJSONPath 按点分 + [n] 路径取值。
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
			if seg.index < 0 || seg.index >= len(node) {
				return nil
			}
			cur = node[seg.index]
		default:
			return nil
		}
	}
	return cur
}

// SetJSONPath 按路径改写值（路径必须已存在——产物声明指向 taskData 已有字段）。
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
			if segs[i].index < 0 || segs[i].index >= len(node) {
				return false
			}
			parent = node[segs[i].index]
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
		if last.index < 0 || last.index >= len(node) {
			return false
		}
		node[last.index] = value
		return true
	default:
		return false
	}
}

type pathSeg struct {
	key   string
	index int
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
