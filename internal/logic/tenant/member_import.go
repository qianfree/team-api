package tenant

import (
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"io"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/utility/crypto"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
)

// ImportResult represents the result of a single row import.
type ImportResult struct {
	Row      int    `json:"row"`
	Username string `json:"username"`
	Status   string `json:"status"` // success, fail, skip
	Error    string `json:"error,omitempty"`
}

// MemberImport parses CSV content, validates, creates an import record.
func (s *sTenant) MemberImport(ctx context.Context, req *v1.TenantMemberImportReq) (*v1.TenantMemberImportRes, error) {
	tenantID := middleware.GetTenantID(ctx)
	creatorID := middleware.GetUserID(ctx)

	// Get CSV from multipart form file
	file := g.RequestFromCtx(ctx).GetUploadFile("file")
	if file == nil {
		return nil, common.NewBadRequestError("请上传CSV文件")
	}
	f, err := file.Open()
	if err != nil {
		return nil, gerror.Wrapf(err, "打开文件失败")
	}
	csvContent, err := io.ReadAll(f)
	if err != nil {
		return nil, gerror.Wrapf(err, "读取文件失败")
	}

	importID, err := startImport(ctx, tenantID, creatorID, file.Filename, csvContent)
	if err != nil {
		return nil, err
	}

	return &v1.TenantMemberImportRes{Data: map[string]any{
		"id": importID,
	}}, nil
}

// ImportRecords returns a paginated list of import records.
func (s *sTenant) ImportRecords(ctx context.Context, req *v1.TenantImportRecordsReq) (*v1.TenantImportRecordsRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	type importRecordRow struct {
		Id           int64       `json:"id"`
		Filename     string      `json:"filename"`
		TotalCount   int         `json:"total_count"`
		SuccessCount int         `json:"success_count"`
		FailCount    int         `json:"fail_count"`
		SkipCount    int         `json:"skip_count"`
		Status       string      `json:"status"`
		ErrorMessage string      `json:"error_message"`
		CreatedAt    *gtime.Time `json:"created_at"`
	}

	var records []importRecordRow
	var total int
	err := dao.TntMemberImports.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("id, filename, total_count, success_count, fail_count, skip_count, status, error_message, created_at").
		OrderDesc("id").
		Page(page, pageSize).
		ScanAndCount(&records, &total, false)
	if err != nil {
		return nil, err
	}
	if records == nil {
		records = []importRecordRow{}
	}

	// Convert to []map[string]any for the response
	list := make([]map[string]any, len(records))
	for i, r := range records {
		list[i] = map[string]any{
			"id":            r.Id,
			"filename":      r.Filename,
			"total_count":   r.TotalCount,
			"success_count": r.SuccessCount,
			"fail_count":    r.FailCount,
			"skip_count":    r.SkipCount,
			"status":        r.Status,
			"error_message": r.ErrorMessage,
			"created_at":    r.CreatedAt,
		}
	}

	return &v1.TenantImportRecordsRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ImportRecordGet returns the status of a member import.
func (s *sTenant) ImportRecordGet(ctx context.Context, req *v1.TenantImportRecordGetReq) (*v1.TenantImportRecordGetRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	var record struct {
		ID           int64  `json:"id"`
		Filename     string `json:"filename"`
		TotalCount   int    `json:"total_count"`
		SuccessCount int    `json:"success_count"`
		FailCount    int    `json:"fail_count"`
		SkipCount    int    `json:"skip_count"`
		Status       string `json:"status"`
		ErrorMessage string `json:"error_message"`
		ResultJSON   string `json:"result_json"`
		CreatedAt    string `json:"created_at"`
	}
	err := dao.TntMemberImports.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&record)
	if err != nil {
		return nil, err
	}
	if record.ID == 0 {
		return nil, common.NewNotFoundError("导入记录")
	}

	result := map[string]any{
		"id":            record.ID,
		"filename":      record.Filename,
		"total_count":   record.TotalCount,
		"success_count": record.SuccessCount,
		"fail_count":    record.FailCount,
		"skip_count":    record.SkipCount,
		"status":        record.Status,
		"error_message": record.ErrorMessage,
	}

	if record.ResultJSON != "" {
		var details []ImportResult
		json.Unmarshal([]byte(record.ResultJSON), &details)
		result["details"] = details
	}

	return &v1.TenantImportRecordGetRes{Data: result}, nil
}

// -- internal helpers --

// startImport parses CSV content, validates, creates an import record.
func startImport(ctx context.Context, tenantID, creatorID int64, filename string, csvContent []byte) (int64, error) {
	reader := csv.NewReader(strings.NewReader(string(csvContent)))
	reader.LazyQuotes = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return 0, gerror.Wrapf(err, "读取CSV表头失败")
	}

	// Validate header
	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.TrimSpace(strings.ToLower(h))] = i
	}
	if _, ok := headerMap["username"]; !ok {
		return 0, common.NewBadRequestError("CSV缺少username列")
	}
	if _, ok := headerMap["role"]; !ok {
		return 0, common.NewBadRequestError("CSV缺少role列")
	}

	// Read all rows
	var rows [][]string
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}
		rows = append(rows, record)
	}

	if len(rows) == 0 {
		return 0, common.NewBadRequestError("CSV无数据行")
	}
	if len(rows) > 500 {
		return 0, gerror.Newf("单次最多导入500条，当前%d条", len(rows))
	}

	// Pre-validate all rows
	results := make([]ImportResult, 0, len(rows))
	seenUsernames := make(map[string]bool)
	seenEmails := make(map[string]bool)

	for i, row := range rows {
		result := ImportResult{Row: i + 1}

		username := getCSVField(row, headerMap, "username")
		email := getCSVField(row, headerMap, "email")
		role := getCSVField(row, headerMap, "role")

		if username == "" {
			result.Status = "fail"
			result.Error = "用户名不能为空"
			result.Username = username
			results = append(results, result)
			continue
		}

		if role != "" && role != "admin" && role != "member" {
			result.Status = "fail"
			result.Error = "角色只能是admin或member"
			result.Username = username
			results = append(results, result)
			continue
		}
		if role == "" {
			role = "member"
		}

		// Check duplicate within CSV
		lowerUsername := strings.ToLower(username)
		if seenUsernames[lowerUsername] {
			result.Status = "skip"
			result.Error = "CSV内用户名重复"
			result.Username = username
			results = append(results, result)
			continue
		}
		seenUsernames[lowerUsername] = true

		if email != "" {
			lowerEmail := strings.ToLower(email)
			if seenEmails[lowerEmail] {
				result.Status = "skip"
				result.Error = "CSV内邮箱重复"
				result.Username = username
				results = append(results, result)
				continue
			}
			seenEmails[lowerEmail] = true
		}

		result.Status = "pending"
		result.Username = username
		results = append(results, result)
	}

	// Check against existing members
	for i := range results {
		if results[i].Status != "pending" {
			continue
		}
		username := results[i].Username
		email := getCSVField(rows[results[i].Row-1], headerMap, "email")

		count, _ := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("username", username).
			Count()
		if count > 0 {
			results[i].Status = "skip"
			results[i].Error = "用户名已存在"
			continue
		}

		if email != "" {
			count, _ = dao.TntUsers.Ctx(ctx).
				Where("tenant_id", tenantID).
				Where("email", strings.ToLower(email)).
				Count()
			if count > 0 {
				results[i].Status = "skip"
				results[i].Error = "邮箱已存在"
			}
		}
	}

	// Check member limit
	pendingCount := 0
	for _, r := range results {
		if r.Status == "pending" {
			pendingCount++
		}
	}

	if pendingCount > 0 {
		currentCount, _ := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("status", "active").
			Count()

		effectiveMaxMembers, _, err := billing.GetTenantEffectiveLimits(ctx, tenantID)
		if err != nil {
			return 0, gerror.Newf("查询租户限制信息失败: %v", err)
		}

		if int(currentCount)+pendingCount > effectiveMaxMembers {
			return 0, gerror.Newf("导入%d条后将超出成员上限%d（当前%d，上限%d）",
				pendingCount, effectiveMaxMembers, currentCount, effectiveMaxMembers)
		}
	}

	// Create import record

	resultJSON, _ := json.Marshal(results)
	importResult, err := dao.TntMemberImports.Ctx(ctx).Data(do.TntMemberImports{
		TenantId:   tenantID,
		Filename:   filename,
		TotalCount: len(rows),
		ResultJson: string(resultJSON),
		Status:     "pending",
		CreatedBy:  creatorID,
	}).Insert()
	if err != nil {
		return 0, gerror.Wrapf(err, "创建导入记录失败")
	}

	importID, _ := importResult.LastInsertId()
	return importID, nil
}

// processImport executes the actual import for pending rows.
func processImport(ctx context.Context, importID int64) error {
	var record struct {
		ID         int64  `json:"id"`
		TenantID   int64  `json:"tenant_id"`
		ResultJSON string `json:"result_json"`
		Status     string `json:"status"`
		CreatedBy  int64  `json:"created_by"`
	}
	err := dao.TntMemberImports.Ctx(ctx).
		Where("id", importID).
		Scan(&record)
	if err != nil || record.ID == 0 {
		return common.NewNotFoundError("导入记录")
	}
	if record.Status != "pending" {
		return common.NewBusinessError(422, "导入记录状态不是pending")
	}

	// Mark as processing
	dao.TntMemberImports.Ctx(ctx).
		Where("id", importID).
		Data(do.TntMemberImports{
			Status: "processing",
		}).Update()

	var results []ImportResult
	if err := json.Unmarshal([]byte(record.ResultJSON), &results); err != nil {
		return gerror.Wrapf(err, "解析导入结果失败")
	}

	successCount := 0
	failCount := 0
	skipCount := 0

	for i := range results {
		if results[i].Status != "pending" {
			if results[i].Status == "skip" {
				skipCount++
			}
			continue
		}

		// Generate random password
		passwordBytes := make([]byte, 16)
		rand.Read(passwordBytes)
		rawPassword := hex.EncodeToString(passwordBytes)[:12]
		passwordHash, err := crypto.HashPassword(rawPassword)
		if err != nil {
			results[i].Status = "fail"
			results[i].Error = "密码生成失败"
			failCount++
			continue
		}

		displayName := results[i].Username

		_, err = dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
			TenantId:     record.TenantID,
			Username:     results[i].Username,
			Email:        "",
			PasswordHash: passwordHash,
			DisplayName:  displayName,
			Role:         "member",
			Status:       "active",
		}).Insert()
		if err != nil {
			results[i].Status = "fail"
			results[i].Error = err.Error()
			failCount++
			continue
		}

		results[i].Status = "success"
		successCount++
	}

	// Update import record
	updatedJSON, _ := json.Marshal(results)
	dao.TntMemberImports.Ctx(ctx).
		Where("id", importID).
		Data(do.TntMemberImports{
			SuccessCount: successCount,
			FailCount:    failCount,
			SkipCount:    skipCount,
			Status:       "completed",
			ResultJson:   string(updatedJSON),
		}).Update()

	return nil
}

// GenerateImportTemplate returns the CSV template content.
func GenerateImportTemplate() string {
	return "username,display_name,email,role,models\nalice,Alice Chen,alice@example.com,member,\nbob,Bob Wang,bob@example.com,admin,gpt-4;claude-3"
}

// getCSVField extracts a field from a CSV row by header map.
func getCSVField(row []string, headerMap map[string]int, fieldName string) string {
	idx, ok := headerMap[fieldName]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}
