package dispatch

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"strconv"
)

// PickHRW 加权 Rendezvous Hash 选择（基线方案 §5.1，纯函数）。
//
//	pick = argmin_c [ -ln(u_c) / W(c) ]，u_c = hash(sessionKey, channelID) → (0,1)
//
// 性质：
//   - 粘性：相同 sessionKey + 相同候选/权重 → 永远同一结果（跨实例确定）；
//   - 按权重比例分布：大量不同 sessionKey 按 W(c) 比例摊到各渠道；
//   - 单调性：增删一个渠道只迁移有限比例的会话。
//
// 权重 <=0 的候选不参与。分数并列按 ChannelID 升序保证全实例一致。
func PickHRW(candidates []ScoredChannel, sessionKey string) *ScoredChannel {
	var best *ScoredChannel
	bestScore := math.Inf(1)
	for i := range candidates {
		c := &candidates[i]
		if c.Weight <= 0 {
			continue
		}
		u := uniformHash(sessionKey, c.ID)
		score := -math.Log(u) / c.Weight
		if best == nil || score < bestScore || (score == bestScore && c.ID < best.ID) {
			best, bestScore = c, score
		}
	}
	return best
}

// uniformHash 把 (sessionKey, channelID) 映射为 (0,1) 上的均匀值。
// 使用 53 位精度构造，且加一避免取到 0（log(0) 为 -Inf）。
func uniformHash(sessionKey string, channelID int64) float64 {
	h := sha256.Sum256([]byte(sessionKey + "\x00" + strconv.FormatInt(channelID, 10)))
	raw := binary.BigEndian.Uint64(h[:8])
	return (float64(raw>>11) + 1) / (float64(uint64(1)<<53) + 1)
}
