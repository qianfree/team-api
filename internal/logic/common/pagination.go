package common

// NormalizePagination 规范化分页参数，确保 page >= 1，pageSize 在 [1, 100] 范围内
func NormalizePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
