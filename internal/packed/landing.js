// 落地页脚本（从 landing.html 外置，全站 CSP script-src 'self' 会拦截内联脚本）
(function () {
    // 开发环境前端 Vite 开发服务器地址（管理后台 :4001 / 租户控制台 :4002），按当前 hostname 动态拼接
    var host = window.location.hostname || 'localhost';
    var admin = document.getElementById('dev-admin');
    var tenant = document.getElementById('dev-tenant');
    if (admin) admin.href = 'http://' + host + ':4001';
    if (tenant) tenant.href = 'http://' + host + ':4002';
})();
