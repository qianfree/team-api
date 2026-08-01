#!/bin/bash
# Turnstile 验证快速测试脚本

echo "=========================================="
echo "  Turnstile 人机验证 - 快速测试"
echo "=========================================="
echo ""

# 检查服务是否运行
echo "1. 检查服务状态..."
if curl -s http://localhost:18888/api/settings/public > /dev/null 2>&1; then
    echo "✅ 服务正在运行"
else
    echo "❌ 服务未运行，请先启动服务"
    echo "   启动命令: ./team-api.exe 或 gf run"
    exit 1
fi

echo ""
echo "2. 检查公共设置..."
SETTINGS=$(curl -s http://localhost:18888/api/settings/public)
echo "$SETTINGS" | jq '.data.settings | {turnstile_enabled, turnstile_site_key, register_email_verification}'

echo ""
echo "3. 当前验证配置："
TURNSTILE_ENABLED=$(echo "$SETTINGS" | jq -r '.data.settings.turnstile_enabled')
TURNSTILE_SITE_KEY=$(echo "$SETTINGS" | jq -r '.data.settings.turnstile_site_key')
EMAIL_VERIFICATION=$(echo "$SETTINGS" | jq -r '.data.settings.register_email_verification')

if [ "$EMAIL_VERIFICATION" = "true" ]; then
    echo "   📧 邮箱验证：启用（优先级最高）"
elif [ "$TURNSTILE_ENABLED" = "true" ] && [ -n "$TURNSTILE_SITE_KEY" ] && [ "$TURNSTILE_SITE_KEY" != "null" ]; then
    echo "   🔒 Turnstile 验证：启用"
    echo "   Site Key: $TURNSTILE_SITE_KEY"
else
    echo "   🎯 滑块验证码：启用（降级模式）"
fi

echo ""
echo "=========================================="
echo "  测试建议"
echo "=========================================="
echo ""
echo "前端测试："
echo "  1. 访问注册页面: http://localhost:3000/tenant/auth/register"
echo "  2. 访问登录页面: http://localhost:3000/tenant/auth/login"
echo ""

if [ "$TURNSTILE_ENABLED" != "true" ] || [ -z "$TURNSTILE_SITE_KEY" ] || [ "$TURNSTILE_SITE_KEY" = "null" ]; then
    echo "⚠️  Turnstile 未配置"
    echo ""
    echo "配置步骤："
    echo "  1. 获取 Cloudflare Turnstile 密钥"
    echo "     https://dash.cloudflare.com/turnstile"
    echo ""
    echo "  2. 通过管理后台配置："
    echo "     http://localhost:3000/admin → 系统设置 → 安全配置"
    echo ""
    echo "  3. 或通过数据库配置："
    echo "     UPDATE sys_options SET value = 'true' WHERE key = 'turnstile_enabled';"
    echo "     UPDATE sys_options SET value = 'YOUR_SITE_KEY' WHERE key = 'turnstile_site_key';"
    echo "     UPDATE sys_options SET value = 'YOUR_SECRET_KEY' WHERE key = 'turnstile_secret_key';"
    echo ""
else
    echo "✅ Turnstile 已配置"
    echo ""
    echo "测试流程："
    echo "  1. 打开注册页面"
    echo "  2. 填写表单"
    echo "  3. 完成 Turnstile 验证（点击复选框）"
    echo "  4. 点击注册"
    echo "  5. 检查是否成功"
    echo ""
fi

echo "详细文档："
echo "  - 实施指南: docs/security/turnstile-implementation-guide.md"
echo "  - 测试清单: docs/security/turnstile-test-checklist.md"
echo ""
