package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/wenlng/go-captcha-assets/resources/imagesv2"
	"github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"

	"github.com/qianfree/team-api/internal/consts"
)

const (
	captchaRedisPrefix    = "captcha"
	captchaVerifiedPrefix = "captcha:verified"
	captchaPadding        = 5
	captchaWidth          = 320
	captchaHeight         = 180
)

var captchaBuilder slide.Builder

func init() {
	graphImages, err := loadGraphImages()
	if err != nil {
		log.Fatalf("[Captcha] failed to load tile assets: %v", err)
	}

	backgroundImages, err := imagesv2.GetImages()
	if err != nil {
		log.Fatalf("[Captcha] failed to load background images: %v", err)
	}

	captchaBuilder = slide.NewBuilder(
		slide.WithImageSize(option.Size{Width: captchaWidth, Height: captchaHeight}),
		slide.WithRangeGraphAnglePos([]option.RangeVal{{Min: 0, Max: 15}}),
		slide.WithRangeGraphSize(option.RangeVal{Min: 50, Max: 80}),
		slide.WithGenGraphNumber(1),
	)
	captchaBuilder.SetResources(
		slide.WithBackgrounds(backgroundImages),
		slide.WithGraphImages(graphImages),
	)
}

func loadGraphImages() ([]*slide.GraphImage, error) {
	assetTiles, err := tiles.GetTiles()
	if err != nil {
		return nil, fmt.Errorf("load tiles: %w", err)
	}

	graphImages := make([]*slide.GraphImage, len(assetTiles))
	for i, t := range assetTiles {
		graphImages[i] = &slide.GraphImage{
			OverlayImage: t.OverlayImage,
			ShadowImage:  t.ShadowImage,
			MaskImage:    t.MaskImage,
		}
	}
	return graphImages, nil
}

// GenerateCaptcha creates a slider captcha, stores the answer in Redis, and returns the images.
func GenerateCaptcha(ctx context.Context) (*CaptchaGenerateResult, error) {
	capt := captchaBuilder.Make()
	captData, err := capt.Generate()
	if err != nil {
		g.Log().Errorf(ctx, "[Captcha] generate failed: %v", err)
		return nil, NewBusinessError(consts.CodeInternalServerError, "验证码生成失败")
	}

	block := captData.GetData()
	captchaKey := uuid.New().String()

	expireSec := getCaptchaExpireSeconds(ctx)
	redisKey := fmt.Sprintf("%s:state:%s", captchaRedisPrefix, captchaKey)

	stateJSON, _ := json.Marshal(map[string]int{"x": block.X, "y": block.Y})
	_, err = g.Redis().Do(ctx, "SETEX", redisKey, int64(expireSec.Seconds()), string(stateJSON))
	if err != nil {
		g.Log().Errorf(ctx, "[Captcha] redis set failed: %v", err)
		return nil, NewBusinessError(consts.CodeInternalServerError, "验证码生成失败")
	}

	masterB64, err := captData.GetMasterImage().ToBase64Data()
	if err != nil {
		g.Log().Errorf(ctx, "[Captcha] master base64 failed: %v", err)
		return nil, NewBusinessError(consts.CodeInternalServerError, "验证码生成失败")
	}

	tileB64, err := captData.GetTileImage().ToBase64Data()
	if err != nil {
		g.Log().Errorf(ctx, "[Captcha] tile base64 failed: %v", err)
		return nil, NewBusinessError(consts.CodeInternalServerError, "验证码生成失败")
	}

	return &CaptchaGenerateResult{
		CaptchaKey:  captchaKey,
		MasterImage: "data:image/jpeg;base64," + masterB64,
		TileImage:   "data:image/png;base64," + tileB64,
		TileY:       block.DY,
	}, nil
}

// CaptchaGenerateResult holds the captcha generation response.
type CaptchaGenerateResult struct {
	CaptchaKey  string `json:"captcha_key"`
	MasterImage string `json:"master_image"`
	TileImage   string `json:"tile_image"`
	TileY       int    `json:"tile_y"`
}

// VerifyCaptcha checks if the user's slide X coordinate matches the stored answer.
// On success, sets a verified flag so CheckCaptchaRequired can pass without re-checking.
func VerifyCaptcha(ctx context.Context, captchaKey string, userX int) (*CaptchaVerifyResult, error) {
	stateKey := fmt.Sprintf("%s:state:%s", captchaRedisPrefix, captchaKey)
	verifiedKey := fmt.Sprintf("%s:%s", captchaVerifiedPrefix, captchaKey)

	result, err := g.Redis().Do(ctx, "GET", stateKey)
	if err != nil || result.IsNil() {
		return &CaptchaVerifyResult{Verified: false}, nil
	}

	jsonStr := result.String()
	if jsonStr == "" {
		return &CaptchaVerifyResult{Verified: false}, nil
	}

	var stored struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &stored); err != nil {
		g.Log().Warningf(ctx, "[Captcha] unmarshal failed: %v", err)
		return &CaptchaVerifyResult{Verified: false}, nil
	}

	verified := slide.Validate(stored.X, stored.Y, userX, stored.Y, captchaPadding)

	if verified {
		// Mark as verified so CheckCaptchaRequired can skip re-check
		expireSec := getCaptchaExpireSeconds(ctx)
		g.Redis().Do(ctx, "SETEX", verifiedKey, int64(expireSec.Seconds()), "1")
	}

	// One-time use: always delete the answer
	_, _ = g.Redis().Do(ctx, "DEL", stateKey)

	return &CaptchaVerifyResult{Verified: verified}, nil
}

// CaptchaVerifyResult holds the captcha verification response.
type CaptchaVerifyResult struct {
	Verified bool `json:"verified"`
}

// CheckCaptchaRequired checks captcha verification. If pre-verified via VerifyCaptcha, passes directly.
func CheckCaptchaRequired(ctx context.Context, _ string, captchaKey string, captchaX int) error {
	if captchaKey == "" {
		return NewBusinessError(consts.CodeCaptchaRequired, consts.MsgCaptchaRequired)
	}

	// Check pre-verification flag first
	verifiedKey := fmt.Sprintf("%s:%s", captchaVerifiedPrefix, captchaKey)
	result, err := g.Redis().Do(ctx, "GET", verifiedKey)
	if err == nil && !result.IsNil() {
		// Already verified — consume the flag and pass
		_, _ = g.Redis().Do(ctx, "DEL", verifiedKey)
		return nil
	}

	// Fallback: direct verification (for non-pre-verified flow)
	stateKey := fmt.Sprintf("%s:state:%s", captchaRedisPrefix, captchaKey)
	result, err = g.Redis().Do(ctx, "GET", stateKey)
	if err != nil || result.IsNil() {
		return NewBusinessError(consts.CodeCaptchaFailed, "验证码已过期，请重新获取")
	}

	jsonStr := result.String()
	var stored struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &stored); err != nil {
		g.Log().Warningf(ctx, "[Captcha] unmarshal failed: %v", err)
		return NewBusinessError(consts.CodeCaptchaFailed, consts.MsgCaptchaFailed)
	}

	verified := slide.Validate(stored.X, stored.Y, captchaX, stored.Y, captchaPadding)

	_, _ = g.Redis().Do(ctx, "DEL", stateKey)

	if !verified {
		return NewBusinessError(consts.CodeCaptchaFailed, consts.MsgCaptchaFailed)
	}

	return nil
}

func getCaptchaExpireSeconds(ctx context.Context) time.Duration {
	secs := Config().GetInt(ctx, "captcha_expire_seconds")
	if secs < 60 {
		secs = 300
	}
	return time.Duration(secs) * time.Second
}
