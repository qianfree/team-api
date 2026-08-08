package billing

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/shopspring/decimal"

	// 注册 GoFrame redis 适配器（生产代码在 main.go 空导入；单测需在此显式注册）
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
)

// miniredis 全局实例：g.Redis() 经 gredis.SetConfig 指向内存 Redis，
// 验证钱包权威化架构下的全部 Lua 脚本（预扣/结算认领/解冻/加款/扣款）。
// miniredis 单线程串行执行命令，脚本内 check-then-act 等价原子，可验证资金守恒。
var mr *miniredis.Miniredis

func TestMain(m *testing.M) {
	var err error
	mr, err = miniredis.Run()
	if err != nil {
		panic(err)
	}
	if err := gredis.SetConfigByMap(map[string]any{
		"address": mr.Addr(),
		"db":      0,
	}); err != nil {
		panic(err)
	}
	code := m.Run()
	mr.Close()
	os.Exit(code)
}

func testCtx() context.Context { return context.Background() }

// readWallet 读取租户钱包余额/冻结（micro → decimal）
func readWallet(t *testing.T, tenantID int64) (balance, frozen decimal.Decimal) {
	t.Helper()
	b, f, exists, err := readWalletHash(testCtx(), tenantID)
	if err != nil {
		t.Fatalf("read wallet hash: %v", err)
	}
	if !exists {
		t.Fatalf("wallet hash missing for tenant %d", tenantID)
	}
	return FromMicro(b), FromMicro(f)
}

// seedWallet 用加款创建指定余额的钱包 hash（顺带校验 CreditWalletRedis 自身）
func seedWallet(t *testing.T, tenantID int64, balance decimal.Decimal) {
	t.Helper()
	ba, _, err := CreditWalletRedis(testCtx(), tenantID, balance)
	if err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	if !ba.Equal(balance) {
		t.Fatalf("seed wallet balance=%s want %s", ba.String(), balance.String())
	}
}

// assertDecimalEqual 断言两个 decimal 相等（金额精度 6 位）
func assertDecimalEqual(t *testing.T, got, want decimal.Decimal, msg string) {
	t.Helper()
	if !got.Equal(want) {
		t.Fatalf("%s: got=%s want=%s", msg, got.String(), want.String())
	}
}

func TestPreDeduct_Basic(t *testing.T) {
	tenant := int64(1)
	seedWallet(t, tenant, NewFromFloat(10))

	ok, err := PreDeduct(testCtx(), tenant, 4, "req-p1", "gpt-4o")
	if err != nil || !ok {
		t.Fatalf("pre-deduct failed: ok=%v err=%v", ok, err)
	}
	bal, frozen := readWallet(t, tenant)
	assertDecimalEqual(t, bal, NewFromFloat(10), "balance unchanged by freeze")
	assertDecimalEqual(t, frozen, NewFromFloat(4), "frozen after prededuct")

	// 幂等：同一 request_id 重复预扣 → 成功且不重复冻结
	ok, err = PreDeduct(testCtx(), tenant, 4, "req-p1", "gpt-4o")
	if err != nil || !ok {
		t.Fatalf("idempotent pre-deduct failed: ok=%v err=%v", ok, err)
	}
	_, frozen = readWallet(t, tenant)
	assertDecimalEqual(t, frozen, NewFromFloat(4), "no double freeze on idempotent")

	// 余额不足：可用 6（10-4）< 7 → 拒绝且不冻结
	ok, err = PreDeduct(testCtx(), tenant, 7, "req-p2", "gpt-4o")
	if ok || err == nil {
		t.Fatalf("insufficient pre-deduct should fail: ok=%v err=%v", ok, err)
	}
	_, frozen = readWallet(t, tenant)
	assertDecimalEqual(t, frozen, NewFromFloat(4), "no freeze on insufficient")
}

func TestSettleClaim_Normal(t *testing.T) {
	tenant := int64(2)
	seedWallet(t, tenant, NewFromFloat(10))
	if ok, _ := PreDeduct(testCtx(), tenant, 4, "req-s1", "gpt-4o"); !ok {
		t.Fatal("prededuct failed")
	}

	claimed, bal, frozen, err := SettleClaim(testCtx(), tenant, NewFromFloat(1.5), []string{"req-s1"})
	if err != nil {
		t.Fatalf("settle claim: %v", err)
	}
	assertDecimalEqual(t, claimed, NewFromFloat(4), "claimed == prededucted")
	assertDecimalEqual(t, bal, NewFromFloat(8.5), "balance after deduct")
	assertDecimalEqual(t, frozen, decimal.Zero, "frozen fully released")
}

func TestSettleClaim_Duplicate(t *testing.T) {
	tenant := int64(3)
	seedWallet(t, tenant, NewFromFloat(10))
	if ok, _ := PreDeduct(testCtx(), tenant, 4, "req-s2", "gpt-4o"); !ok {
		t.Fatal("prededuct failed")
	}
	if _, _, _, err := SettleClaim(testCtx(), tenant, NewFromFloat(1.5), []string{"req-s2"}); err != nil {
		t.Fatalf("first settle: %v", err)
	}

	// 重复结算：预扣 hash 已随认领删除 → claimed=0，只按补扣继续扣 balance
	claimed, bal, frozen, err := SettleClaim(testCtx(), tenant, NewFromFloat(0.5), []string{"req-s2"})
	if err != nil {
		t.Fatalf("duplicate settle claim: %v", err)
	}
	assertDecimalEqual(t, claimed, decimal.Zero, "duplicate claims nothing")
	assertDecimalEqual(t, bal, NewFromFloat(8), "only supplement deducted")
	assertDecimalEqual(t, frozen, decimal.Zero, "frozen untouched")
}

func TestSettleClaim_MultiKeyAdjust(t *testing.T) {
	tenant := int64(4)
	seedWallet(t, tenant, NewFromFloat(10))
	if ok, _ := PreDeduct(testCtx(), tenant, 3, "req-t1", "midjourney"); !ok {
		t.Fatal("prededuct failed")
	}
	if ok, _ := PreDeduct(testCtx(), tenant, 1, "req-t1_adjust", "midjourney"); !ok {
		t.Fatal("adjust prededuct failed")
	}

	claimed, bal, frozen, err := SettleClaim(testCtx(), tenant, NewFromFloat(2), []string{"req-t1", "req-t1_adjust"})
	if err != nil {
		t.Fatalf("multi-key settle claim: %v", err)
	}
	assertDecimalEqual(t, claimed, NewFromFloat(4), "sum of both prededucts claimed")
	assertDecimalEqual(t, bal, NewFromFloat(8), "balance after cost")
	assertDecimalEqual(t, frozen, decimal.Zero, "all frozen released")
}

func TestUnfreeze_Idempotent(t *testing.T) {
	tenant := int64(5)
	seedWallet(t, tenant, NewFromFloat(10))
	if ok, _ := PreDeduct(testCtx(), tenant, 2, "req-u1", "gpt-4o"); !ok {
		t.Fatal("prededuct failed")
	}

	amt, err := UnfreezePreDeduct(testCtx(), tenant, "req-u1")
	if err != nil {
		t.Fatalf("unfreeze: %v", err)
	}
	assertDecimalEqual(t, amt, NewFromFloat(2), "unfreeze releases prededucted amount")
	_, frozen := readWallet(t, tenant)
	assertDecimalEqual(t, frozen, decimal.Zero, "frozen released")

	// 幂等：第二次认领不到 → 0，不二次释放
	amt2, err := UnfreezePreDeduct(testCtx(), tenant, "req-u1")
	if err != nil {
		t.Fatalf("second unfreeze: %v", err)
	}
	assertDecimalEqual(t, amt2, decimal.Zero, "idempotent second unfreeze")

	// 不存在的预扣 → 0
	amt3, err := UnfreezePreDeduct(testCtx(), tenant, "req-missing")
	if err != nil {
		t.Fatalf("missing unfreeze: %v", err)
	}
	assertDecimalEqual(t, amt3, decimal.Zero, "missing prededuct no-op")
}

func TestCreditDebitWallet(t *testing.T) {
	tenant := int64(6)
	seedWallet(t, tenant, NewFromFloat(10))

	// 扣款门槛：可用 10 < 12 → 拒绝且不变更
	_, _, ok, err := DebitWalletRedis(testCtx(), tenant, NewFromFloat(12))
	if err != nil {
		t.Fatalf("debit err: %v", err)
	}
	if ok {
		t.Fatal("debit beyond available must fail")
	}
	bal, _ := readWallet(t, tenant)
	assertDecimalEqual(t, bal, NewFromFloat(10), "no change on failed debit")

	// 正常扣款
	after, _, ok, err := DebitWalletRedis(testCtx(), tenant, NewFromFloat(3))
	if err != nil || !ok {
		t.Fatalf("debit failed: ok=%v err=%v", ok, err)
	}
	assertDecimalEqual(t, after, NewFromFloat(7), "debit reduces balance")

	// 加款（负数=扣回补偿）
	after, _, err = CreditWalletRedis(testCtx(), tenant, NewFromFloat(-3))
	if err != nil {
		t.Fatalf("credit: %v", err)
	}
	assertDecimalEqual(t, after, NewFromFloat(4), "negative credit reduces balance")
}

// TestWalletConcurrency 并发守恒：N 并发预扣+结算后，frozen 归零、balance 精确扣减
func TestWalletConcurrency(t *testing.T) {
	tenant := int64(10)
	seedWallet(t, tenant, NewFromFloat(100))

	const n = 50
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := testCtx()
			rid := "req-c" + strconv.Itoa(idx)
			ok, err := PreDeduct(ctx, tenant, 1, rid, "gpt-4o")
			if err != nil || !ok {
				errCh <- err
				return
			}
			if _, _, _, err := SettleClaim(ctx, tenant, NewFromFloat(0.5), []string{rid}); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent wallet op failed: %v", err)
	}

	bal, frozen := readWallet(t, tenant)
	assertDecimalEqual(t, bal, NewFromFloat(100).Sub(NewFromFloat(0.5).Mul(NewFromFloat(n))), "balance conserved (100 - n*0.5)")
	assertDecimalEqual(t, frozen, decimal.Zero, "frozen converges to zero")
}

// TestPredeductSweep_RecomputesFrozen 孤儿清扫重算：某预扣 hash 丢失（TTL 过期/崩溃残留）后，
// frozen 仍含其金额 → sweep 重算为幸存 hash 之和并清理活跃集合死亡成员。
func TestPredeductSweep_RecomputesFrozen(t *testing.T) {
	tenant := int64(8)
	seedWallet(t, tenant, NewFromFloat(10))
	if ok, _ := PreDeduct(testCtx(), tenant, 2, "req-sw1", "gpt-4o"); !ok {
		t.Fatal("prededuct 1 failed")
	}
	if ok, _ := PreDeduct(testCtx(), tenant, 3, "req-sw2", "gpt-4o"); !ok {
		t.Fatal("prededuct 2 failed")
	}
	_, frozen := readWallet(t, tenant)
	assertDecimalEqual(t, frozen, NewFromFloat(5), "frozen after two prededucts")

	// 模拟孤儿：直接删除 req-sw2 的预扣 hash（不释放冻结）
	if !mr.Del(PreDeductRedisKeyPrefix + "req-sw2") {
		t.Fatal("simulate orphan: prededuct hash not found")
	}

	PredeductSweep(testCtx())

	_, frozen = readWallet(t, tenant)
	assertDecimalEqual(t, frozen, NewFromFloat(2), "frozen recomputed to surviving prededucts")

	// 孤儿 request 移出活跃集合
	activeSetKey := "prededuct_active:8"
	members, err := mr.SMembers(activeSetKey)
	if err != nil {
		t.Fatalf("read active set: %v", err)
	}
	if len(members) != 1 || members[0] != "req-sw1" {
		t.Fatalf("active set after sweep = %v, want [req-sw1]", members)
	}
}

func TestCleanupPreDeduct_RemovesHash(t *testing.T) {
	tenant := int64(7)
	seedWallet(t, tenant, NewFromFloat(10))
	if ok, _ := PreDeduct(testCtx(), tenant, 1, "req-cl1", "gpt-4o"); !ok {
		t.Fatal("prededuct failed")
	}
	CleanupPreDeduct(testCtx(), tenant, "req-cl1")
	if _, exists := GetPreDeductAmount(testCtx(), "req-cl1"); exists {
		t.Fatal("prededuct hash should be cleaned")
	}
}
